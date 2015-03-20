/*
Copyright 2014 The Camlistore Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
Package dockertest contains helper functions for setting up and tearing down docker containers to aid in testing.
*/
package dockertest

import (
    "bytes"
    "database/sql"
    "encoding/json"
    "errors"
    "fmt"
    "log"
    "os/exec"
    "strings"
    "testing"
    "time"

    "camlistore.org/pkg/netutil"
    "math/rand"
)

// Debug, if set, prevents any container from being removed.
var Debug bool

// Boot2DockerAvailable, if true, uses boot2docker to run docker commands (for running tests on Windows and Mac OS)
var Boot2DockerAvailable bool

/// runLongTest checks all the conditions for running a docker container
// based on image.
func runLongTest(t *testing.T, image string) {
    Boot2DockerAvailable = false
    if testing.Short() {
        log.Print("skipping in short mode")
        t.SkipNow()
    }
    if haveBoot2Docker() {
        Boot2DockerAvailable = startBoot2Docker()
        if !Boot2DockerAvailable {
            log.Print("skipping test; 'boot2docker' command failed")
            t.SkipNow()
        }
    } else if !haveDocker() {
        log.Print("skipping test; 'docker' command not found")
        t.SkipNow()
    }
    if ok, err := haveImage(image); !ok || err != nil {
        if err != nil {
            log.Printf("Error running docker to check for %s: %v", image, err)
            t.SkipNow()
        }
        log.Printf("Pulling docker image %s ...", image)
        if err := Pull(image); err != nil {
            log.Printf("Error pulling %s: %v", image, err)
            t.SkipNow()
        }
    }
}

func runDockerCommand(command string, args ...string) *exec.Cmd {
    if Boot2DockerAvailable {
        cmd := exec.Command("boot2docker", append([]string{"ssh", command}, args...)...)
        return cmd
    }
    return exec.Command(command, args...)
}

// haveBoot2Docker returns whether the "docker" command was found.
func haveBoot2Docker() bool {
    _, err := exec.LookPath("boot2docker")
    return err == nil
}

// haveBoot2Docker returns whether the "docker" command was found.
func startBoot2Docker() bool {
    _, err := exec.Command("boot2docker", "start").Output()
    return err == nil
}

// haveDocker returns whether the "docker" command was found.
func haveDocker() bool {
    _, err := exec.LookPath("docker")
    return err == nil
}

func haveImage(name string) (ok bool, err error) {
    out, err := runDockerCommand("docker", "images", "--no-trunc").Output()
    if err != nil {
        return false, err
    }
    return bytes.Contains(out, []byte(name)), nil
}

func run(args ...string) (containerID string, err error) {
    cmd := runDockerCommand("docker", append([]string{"run"}, args...)...)
    var stdout, stderr bytes.Buffer
    cmd.Stdout, cmd.Stderr = &stdout, &stderr
    if err = cmd.Run(); err != nil {
        err = fmt.Errorf("%v%v", stderr.String(), err)
        return
    }
    containerID = strings.TrimSpace(stdout.String())
    if containerID == "" {
        return "", errors.New("unexpected empty output from `docker run`")
    }
    return
}

func KillContainer(container string) error {
    return runDockerCommand("docker", "kill", container).Run()
}

// Pull retrieves the docker image with 'docker pull'.
func Pull(image string) error {
    out, err := runDockerCommand("docker", "pull", image).CombinedOutput()
    if err != nil {
        err = fmt.Errorf("%v: %s", err, out)
    }
    return err
}

// IP returns the IP address of the container.
func IP(containerID string) (string, error) {
    out, err := runDockerCommand("docker", "inspect", containerID).Output()
    if err != nil {
        return "", err
    }
    type networkSettings struct {
        IPAddress string
    }
    type container struct {
        NetworkSettings networkSettings
    }
    var c []container
    if err := json.NewDecoder(bytes.NewReader(out)).Decode(&c); err != nil {
        return "", err
    }
    if len(c) == 0 {
        return "", errors.New("no output from docker inspect")
    }
    if ip := c[0].NetworkSettings.IPAddress; ip != "" {
        return ip, nil
    }
    return "", errors.New("could not find an IP. Not running?")
}

type ContainerID string

func (c ContainerID) IP() (string, error) {
    return IP(string(c))
}

func (c ContainerID) Kill() error {
    return KillContainer(string(c))
}

// Remove runs "docker rm" on the container
func (c ContainerID) Remove() error {
    if Debug {
        return nil
    }
    return runDockerCommand("docker", "rm", "-v", string(c)).Run()
}

// KillRemove calls Kill on the container, and then Remove if there was
// no error. It logs any error to t.
func (c ContainerID) KillRemove(t *testing.T) {
    if err := c.Kill(); err != nil {
        t.Log(err)
        return
    }
    if err := c.Remove(); err != nil {
        t.Log(err)
    }
}

// lookup retrieves the ip address of the container, and tries to reach
// before timeout the tcp address at this ip and given port.
func (c ContainerID) lookup(port int, timeout time.Duration) (ip string, err error) {
    if Boot2DockerAvailable {
        var out []byte
        out, err = exec.Command("boot2docker", "ip").Output()
        ip = strings.TrimSpace(string(out))
    } else {
        ip, err = c.IP()
    }
    if err != nil {
        err = fmt.Errorf("error getting IP: %v", err)
        return
    }
    addr := fmt.Sprintf("%s:%d", ip, port)
    err = netutil.AwaitReachable(addr, timeout)
    return
}

// setupContainer sets up a container, using the start function to run the given image.
// It also looks up the IP address of the container, and tests this address with the given
// port and timeout. It returns the container ID and its IP address, or makes the test
// fail on error.
func setupContainer(t *testing.T, image string, port int, timeout time.Duration, start func() (string, error)) (c ContainerID, ip string) {
    runLongTest(t, image)

    containerID, err := start()
    if err != nil {
        t.Fatalf("docker run: %v", err)
    }
    c = ContainerID(containerID)
    ip, err = c.lookup(port, timeout)
    if err != nil {
        c.KillRemove(t)
        log.Print("Skipping test for container %v: %v", c, err)
        t.SkipNow()
    }
    return c, ip
}

const (
    mongoImage       = "dockerfile/mongodb"
    mysqlImage       = "orchardup/mysql"
    MySQLUsername    = "root"
    MySQLPassword    = "root"
    postgresImage    = "nornagon/postgres"
    PostgresUsername = "docker" // set up by the dockerfile of postgresImage
    PostgresPassword = "docker" // set up by the dockerfile of postgresImage
)

func randInt(min int, max int) int {
    rand.Seed(time.Now().UTC().UnixNano())
    return min + rand.Intn(max-min)
}

// SetupMongoContainer sets up a real MongoDB instance for testing purposes,
// using a Docker container. It returns the container ID and its IP address,
// or makes the test fail on error.
// Currently using https://index.docker.io/u/robinvdvleuten/mongo/
func SetupMongoContainer(t *testing.T) (c ContainerID, ip string, port int) {
    port = randInt(1024, 49150)
    c, ip = setupContainer(t, mongoImage, port, 10*time.Second, func() (string, error) {
        res, err := run("-d", "-p", fmt.Sprintf("%d:%d", port, 27017), mongoImage)
        return res, err
    })
    return
}

// SetupMySQLContainer sets up a real MySQL instance for testing purposes,
// using a Docker container. It returns the container ID and its IP address,
// or makes the test fail on error.
// Currently using https://index.docker.io/u/orchardup/mysql/
func SetupMySQLContainer(t *testing.T, dbname string) (c ContainerID, ip string, port int) {
    port = randInt(1024, 49150)
    c, ip = setupContainer(t, mysqlImage, port, 10*time.Second, func() (string, error) {
        return run("-d", "-p", fmt.Sprintf("%d:%d", port, 3306), "-e", "MYSQL_ROOT_PASSWORD="+MySQLPassword, "-e", "MYSQL_DATABASE="+dbname, mysqlImage)
    })
    return
}

// SetupPostgreSQLContainer sets up a real PostgreSQL instance for testing purposes,
// using a Docker container. It returns the container ID and its IP address,
// or makes the test fail on error.
// Currently using https://index.docker.io/u/nornagon/postgres
func SetupPostgreSQLContainer(t *testing.T, dbname string) (c ContainerID, ip string, port int) {
    port = randInt(1024, 49150)
    c, ip = setupContainer(t, postgresImage, port, 15*time.Second, func() (string, error) {
        return run("-d", "-p", fmt.Sprintf("%d:%d", port, 5432), postgresImage)
    })
    cleanupAndDie := func(err error) {
        c.KillRemove(t)
        t.Fatal(err)
    }
    rootdb, err := sql.Open("postgres",
    fmt.Sprintf("user=%s password=%s host=%s dbname=postgres sslmode=disable", PostgresUsername, PostgresPassword, ip))
    if err != nil {
        cleanupAndDie(fmt.Errorf("Could not open postgres rootdb: %v", err))
    }
    if _, err := sqlExecRetry(rootdb,
    "CREATE DATABASE "+dbname+" LC_COLLATE = 'C' TEMPLATE = template0",
    50); err != nil {
        cleanupAndDie(fmt.Errorf("Could not create database %v: %v", dbname, err))
    }
    return
}

// sqlExecRetry keeps calling http://golang.org/pkg/database/sql/#DB.Exec on db
// with stmt until it succeeds or until it has been tried maxTry times.
// It sleeps in between tries, twice longer after each new try, starting with
// 100 milliseconds.
func sqlExecRetry(db *sql.DB, stmt string, maxTry int) (sql.Result, error) {
    if maxTry <= 0 {
        return nil, errors.New("did not try at all")
    }
    interval := 100 * time.Millisecond
    try := 0
    var err error
    var result sql.Result
    for {
        result, err = db.Exec(stmt)
        if err == nil {
            return result, nil
        }
        try++
        if try == maxTry {
            break
        }
        time.Sleep(interval)
        interval *= 2
    }
    return result, fmt.Errorf("failed %v times: %v", try, err)
}
