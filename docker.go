package dockertest

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

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"math/rand"
	"regexp"

	"github.com/pborman/uuid"
)

/// runLongTest checks all the conditions for running a docker container
// based on image.
func runLongTest(image string) error {
	DockerMachineAvailable = false
	if haveDockerMachine() {
		DockerMachineAvailable = true
		if !startDockerMachine() {
			log.Printf(`Starting docker machine "%s" failed. This could be because the image is already running or because the image does not exist. Tests will fail if the image does not exist.`, DockerMachineName)
		}
	} else if !haveDocker() {
		return errors.New("Neither 'docker' nor 'docker-machine' available on this system.")
	}
	if ok, err := haveImage(image); !ok || err != nil {
		if err != nil {
			return fmt.Errorf("Error checking for docker image %s: %v", image, err)
		}
		log.Printf("Pulling docker image %s ...", image)
		if err := Pull(image); err != nil {
			return fmt.Errorf("Error pulling %s: %v", image, err)
		}
	}
	return nil
}

func runDockerCommand(command string, args ...string) *exec.Cmd {
	if DockerMachineAvailable {
		command = "/usr/local/bin/" + strings.Join(append([]string{command}, args...), " ")
		cmd := exec.Command("docker-machine", "ssh", DockerMachineName, command)
		return cmd
	}
	return exec.Command(command, args...)
}

// haveDockerMachine returns whether the "docker" command was found.
func haveDockerMachine() bool {
	_, err := exec.LookPath("docker-machine")
	return err == nil
}

// startDockerMachine starts the docker machine and returns false if the command failed to execute
func startDockerMachine() bool {
	_, err := exec.Command("docker-machine", "start", DockerMachineName).Output()
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
	var stdout, stderr bytes.Buffer
	validID := regexp.MustCompile(`^([a-zA-Z0-9]+)$`)
	cmd := runDockerCommand("docker", append([]string{"run"}, args...)...)

	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err = cmd.Run(); err != nil {
		err = fmt.Errorf("Error running docker\nStdOut: %s\nStdErr: %s\nError: %v\n\n", stdout.String(), stderr.String(), err)
		return
	}
	containerID = strings.TrimSpace(string(stdout.String()))
	if !validID.MatchString(containerID) {
		return "", fmt.Errorf("Error running docker: %s", containerID)
	}
	if containerID == "" {
		return "", errors.New("Unexpected empty output from `docker run`")
	}
	return containerID, nil
}

// KillContainer runs docker kill on a container.
func KillContainer(container string) error {
	if container != "" {
		return runDockerCommand("docker", "kill", container).Run()
	}
	return nil
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

// setupContainer sets up a container, using the start function to run the given image.
// It also looks up the IP address of the container, and tests this address with the given
// port and timeout. It returns the container ID and its IP address, or makes the test
// fail on error.
func setupContainer(image string, port int, timeout time.Duration, start func() (string, error)) (c ContainerID, ip string, err error) {
	err = runLongTest(image)
	if err != nil {
		return "", "", err
	}

	containerID, err := start()
	if err != nil {
		return "", "", err
	}

	c = ContainerID(containerID)
	ip, err = c.lookup(port, timeout)
	if err != nil {
		c.KillRemove()
		return "", "", err
	}
	return c, ip, nil
}

func randInt(min int, max int) int {
	rand.Seed(time.Now().UTC().UnixNano())
	return min + rand.Intn(max-min)
}

// SetupMongoContainer sets up a real MongoDB instance for testing purposes,
// using a Docker container. It returns the container ID and its IP address,
// or makes the test fail on error.
func SetupMongoContainer(args ...string) (c ContainerID, ip string, port int, err error) {
	return SetupContainer(mongoImage, 27017, args...)
}

// SetupMySQLContainer sets up a real MySQL instance for testing purposes,
// using a Docker container. It returns the container ID and its IP address,
// or makes the test fail on error.
func SetupMySQLContainer(args ...string) (c ContainerID, ip string, port int, err error) {
	return SetupContainerWithEnv(mysqlImage, 3306, fmt.Sprintf("MYSQL_ROOT_PASSWORD=%s", MySQLPassword), args...)
}

// SetupPostgreSQLContainer sets up a real PostgreSQL instance for testing purposes,
// using a Docker container. It returns the container ID and its IP address,
// or makes the test fail on error.
func SetupPostgreSQLContainer(args ...string) (c ContainerID, ip string, port int, err error) {
	return SetupContainerWithEnv(postgresImage, 5432, fmt.Sprintf("POSTGRES_PASSWORD=%s", PostgresPassword), args...)
}

// SetupElasticSearchContainer sets up a real ElasticSearch instance for testing purposes
// using a Docker container. It returns the container ID and its IP address,
// or makes the test fail on error.
func SetupElasticSearchContainer() (c ContainerID, ip string, port int, err error) {
	return SetupContainer(elasticsearchImage, 9200)
}

// SetupRedisContainer sets up a real Redis instance for testing purposes
// using a Docker container. It returns the container ID and its IP address,
// or makes the test fail on error.
func SetupRedisContainer() (c ContainerID, ip string, port int, err error) {
	return SetupContainer(redisImage, 6379)
}

// SetupNatsContainer sets up a real natsd instance for testing purposes
// using Docker container.
func SetupNatsContainer() (c ContainerID, ip string, port int, err error) {
	return SetupContainer(natsImage, 4222)
}

// SetupFluentdContainer sets up a real natsd instance for testing purposes
// using Docker container.
func SetupFluentdContainer() (c ContainerID, ip string, port int, err error) {
	return SetupContainer(fluentdImage, 24224)
}

// SetupContainer runs docker instance and returns port.
func SetupContainer(image string, containerPort int, args ...string) (c ContainerID, ip string, port int, err error) {
	return SetupContainerWithEnv(image, containerPort, "", args...)
}

// SetupContainerWithEnv runs docker instance with env variable and returns port.
func SetupContainerWithEnv(image string, containerPort int, env string, args ...string) (c ContainerID, ip string, port int, err error) {
	log.Printf("setup container %s", image)
	port = randInt(1024, 49150)
	forward := fmt.Sprintf("%d:%d", port, containerPort)
	if BindDockerToLocalhost != "" {
		forward = "127.0.0.1:" + forward
	}
	c, ip, err = setupContainer(image, port, 60*time.Second, func() (string, error) {

		rargs := []string{"--name", uuid.New(), "-d", "-P", "-p", forward}
		if env != "" {
			rargs = append(rargs, "-e", env)
		}
		rargs = append(rargs, image)
		rargs = append(rargs, args...)
		return run(rargs...)
	})
	return
}
