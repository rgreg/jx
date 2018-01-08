package jenkins

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"

	"github.com/jenkins-x/golang-jenkins"
	"github.com/jenkins-x/jx/pkg/util"
	"gopkg.in/AlecAivazis/survey.v1"
)

func GetJenkinsClient(url string, batch bool, configService *JenkinsConfigService) (*gojenkins.Jenkins, error) {
	if url == "" {
		return nil, errors.New("no JENKINS_URL environment variable is set nor could a Jenkins service be found in the current namespace!\n")
	}
	tokenUrl := util.UrlJoin(url, "/me/configure")

	auth := CreateJenkinsAuthFromEnvironment()
	username := auth.Username
	var err error
	config := JenkinsConfig{}

	showForm := false
	if auth.IsInvalid() {
		// lets try load the current auth
		config, err = configService.LoadConfig()
		if err != nil {
			return nil, err
		}
		auths := config.FindAuths(url)
		if len(auths) > 1 {
			// TODO choose an auth
		}
		showForm = true
		a := config.FindAuth(url, username)
		if a != nil {
			if a.IsInvalid() {
				auth, err = EditJenkinsAuth(url, configService, &config, a, tokenUrl)
				if err != nil {
					return nil, err
				}
			} else {
				auth = *a
			}
		} else {
			// lets create a new Auth
			auth, err = EditJenkinsAuth(url, configService, &config, &auth, tokenUrl)
			if err != nil {
				return nil, err
			}
		}
	}

	if auth.IsInvalid() {
		if showForm {
			return nil, fmt.Errorf("No valid Username and API Token specified for Jenkins server: %s\n", url)
		} else {
			fmt.Println("No $JENKINS_USERNAME and $JENKINS_TOKEN environment variables defined!\n")
			fmt.Printf("Please go to %s and click 'Show API Token' to get your API Token\n", tokenUrl)
			if batch {
				fmt.Println("Then run this command on your terminal and try again:\n")
				fmt.Println("export JENKINS_TOKEN=myApiToken\n")
				return nil, errors.New("No environment variables (JENKINS_USERNAME and JENKINS_TOKEN) or JENKINS_BEARER_TOKEN defined")
			}
		}
	}

	jauth := &gojenkins.Auth{
		Username:    auth.Username,
		ApiToken:    auth.ApiToken,
		BearerToken: auth.BearerToken,
	}
	jenkins := gojenkins.NewJenkins(jauth, url)

	// handle insecure TLS for minishift
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}}
	jenkins.SetHTTPClient(httpClient)
	return jenkins, nil
}

func EditJenkinsAuth(url string, configService *JenkinsConfigService, config *JenkinsConfig, auth *JenkinsAuth, tokenUrl string) (JenkinsAuth, error) {
	fmt.Printf("\nTo be able to connect to the Jenkins server we need a username and API Token\n\n")
	fmt.Printf("Please go to %s and click 'Show API Token' to get your API Token\n", tokenUrl)
	fmt.Printf("Then COPY the API token so that you can paste it into the form below:\n\n")

	answers := *auth

	// default the user name
	defaultUsername := config.DefaultUsername
	if defaultUsername == "" {
		defaultUsername = "admin"
	}
	if answers.Username == "" {
		answers.Username = defaultUsername
	}

	var qs = []*survey.Question{
		{
			Name: "username",
			Prompt: &survey.Input{
				Message: "Jenkins user name:",
				Default: answers.Username,
			},
			Validate: survey.Required,
		},
		{
			Name: "apiToken",
			Prompt: &survey.Input{
				Message: "Jenkins API Token:",
				Default: answers.ApiToken,
			},
			Validate: survey.Required,
		},
	}
	err := survey.Ask(qs, &answers)
	fmt.Println()
	if err != nil {
		return answers, err
	}
	config.SetAuth(url, answers)
	config.DefaultUsername = answers.Username
	err = configService.SaveConfig(config)
	return answers, err
}
