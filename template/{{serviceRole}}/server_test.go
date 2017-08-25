package main_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	yaml "gopkg.in/yaml.v2"

	"github.com/DATA-DOG/godog"
	"github.com/Originate/exocom/go/exocom-mock"
	"github.com/Originate/exocom/go/structs"
	execplus "github.com/Originate/go-execplus"
)

type ServiceConfig struct {
	Type string `yaml:type`
}

func getServiceConfig() (*ServiceConfig, error) {
	config := &ServiceConfig{}
	configBytes, err := ioutil.ReadFile("service.yml")
	if err != nil {
		return nil, fmt.Errorf("Error reading service.yml", err)
	}
	err = yaml.Unmarshal(configBytes, &config)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshaling service.yml", err)
	}
	return config, nil
}

func getRole() (string, error) {
	config, err := getServiceConfig()
	if err != nil {
		return "", err
	}
	return config.Type, nil
}

func newExocomMock(port int) *exocomMock.ExoComMock {
	exocom := exocomMock.New()
	go func() {
		err := exocom.Listen(port)
		if err != nil && err != http.ErrServerClosed {
			panic(fmt.Errorf("Error listening on exocom", err))
		}
	}()
	return exocom
}

func FeatureContext(s *godog.Suite) {
	var exocom *exocomMock.ExoComMock
	var role string
	var cmdPlus *execplus.CmdPlus
	port := 4100

	s.BeforeSuite(func() {
		var err error
		exocom = newExocomMock(port)
		role, err = getRole()
		if err != nil {
			panic(err)
		}
	})

	s.BeforeScenario(func(arg1 interface{}) {
		cmdPlus = nil
	})

	s.AfterScenario(func(interface{}, error) {
		exocom.Reset()
		if cmdPlus != nil {
			err := cmdPlus.Kill()
			if err != nil {
				panic(fmt.Errorf("Error when killing the service command: %v", err))
			}
		}
	})

	s.AfterSuite(func() {
		err := exocom.Close()
		if err != nil {
			panic(fmt.Errorf("Error closing exocom", err))
		}
	})

	s.Step(`^an ExoCom server$`, func() error {
		return nil // Empty step as this is done in the BeforeSuite
	})

	s.Step(`^an instance of this service$`, func() error {
		cmdPlus = execplus.NewCmdPlus("go", "run", "server.go")
		env := append(os.Environ(), "EXOCOM_HOST=localhost", fmt.Sprintf("EXOCOM_PORT=%d", port), fmt.Sprintf("ROLE=%d", role))
		cmdPlus.SetEnv(env)
		err := cmdPlus.Start()
		if os.Getenv("DEBUG") != "" {
			go func() {
				outputChannel, _ := cmdPlus.GetOutputChannel()
				for {
					outputChunk := <-outputChannel
					fmt.Println(outputChunk.Chunk)
				}
			}()
		}
		if err != nil {
			return err
		}
		_, err = exocom.WaitForConnection()
		return err
	})

	s.Step(`^receiving the "([^"]*)" command$`, func(name string) error {
		return exocom.Send(structs.Message{Name: name})
	})

	s.Step(`^this service replies with a "([^"]*)" message$`, func(name string) error {
		_, err := exocom.WaitForMessageWithName(name)
		return err
	})
}

func TestMain(m *testing.M) {
	var paths []string
	var format string
	if len(os.Args) == 3 && os.Args[1] == "--" {
		format = "pretty"
		paths = append(paths, os.Args[2])
	} else {
		format = "progress"
		paths = append(paths, "features")
	}
	status := godog.RunWithOptions("godogs", func(s *godog.Suite) {
		FeatureContext(s)
	}, godog.Options{
		Format:        format,
		NoColors:      false,
		StopOnFailure: true,
		Paths:         paths,
	})

	os.Exit(status)
}
