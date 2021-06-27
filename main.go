package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/bradleyfalzon/ghinstallation"
	v3 "github.com/google/go-GitHub/v32/GitHub"
	"github.com/joho/godotenv"
	ghwebhooks "gopkg.in/go-playground/webhooks.v5/GitHub"
)

var (
	installationID int64
	itr            *ghinstallation.Transport
)

func envVar(key string) string {
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	return os.Getenv(key)
}

func main() {
	orgID := envVar("ORG_ID")
	certPath := envVar("CERT_PATH")
	appString := envVar("APP_ID")
	// ghinstallation requires int64
	appID, err := strconv.ParseInt(appString, 10, 64)
	if err == nil {
		fmt.Printf("%d of type %T", appID, appID)
	}

	atr, err := ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, appID, certPath)
	if err != nil {
		log.Fatal("error creating GitHub app client")
	}

	installation, _, err := v3.NewClient(&http.Client{Transport: atr}).Apps.FindOrganizationInstallation(context.TODO(), orgID)
	if err != nil {
		log.Fatalf("error finding organization installation: %v", err)
	}

	installationID = installation.GetID()
	itr = ghinstallation.NewFromAppsTransport(atr, installationID)

	log.Printf("successfully initialized GitHub app client, installation-id:%s expected-events:%v\n", installationID, installation.Events)

	// startup our web server!
	http.HandleFunc("/GitHub", Handle)
	err = http.ListenAndServe("0.0.0.0:3210", nil)
	if err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func Handle(response http.ResponseWriter, request *http.Request) {
	webhookSecret := envVar("WEBHOOK_SECRET")
	hook, err := ghwebhooks.New(ghwebhooks.Options.Secret(webhookSecret))
	if err != nil {
		log.Println("Error setting up gh webhooks client")
		return
	}

	payload, err := hook.Parse(request, []ghwebhooks.Event{ghwebhooks.ReleaseEvent}...)
	if err != nil {
		if err == ghwebhooks.ErrEventNotFound {
			log.Printf("received unregistered GitHub event: %v\n", err)
			response.WriteHeader(http.StatusOK)
		} else {
			log.Printf("received malformed GitHub event: %v\n", err)
			response.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	switch payload := payload.(type) {
	case ghwebhooks.ReleasePayload:
		log.Println("received release event")
		go processReleaseEvent(&payload)
	case ghwebhooks.PullRequestsPayload:
		log.Println("received PR event")
		go processPrEvent(&payload)
	default:
		log.Println("missing github webhook handler")
	}

	response.WriteHeader(http.StatusOK)
}

func GetV3Client() *v3.Client {
	return v3.NewClient(&http.Client{Transport: itr})
}

func processReleaseEvent(p *ghwebhooks.ReleasePayload) {
	// TODO: make these env vars global
	orgID := envVar("ORG_ID")
	pr, _, err := GetV3Client().PullRequests.Create(context.TODO(), orgID, envVar("REPO_NAME"), &v3.NewPullRequest{
		Title:               v3.String("Hello pull request!"),
		Head:                v3.String("develop"),
		Base:                v3.String("master"),
		Body:                v3.String("This is an automatically created PR."),
		MaintainerCanModify: v3.Bool(true),
	})
	if err != nil {
		if !strings.Contains(err.Error(), "A pull request already exists") {
			log.Printf("error creating pull request: %v\n", err)
		}
	} else {
		log.Printf("created pull request: %s", pr.GetURL())
	}
}

func processPrEvent(p *ghwebhooks.PullRequestsPayload) {
	// TODO: make these env vars global
	orgID := envVar("ORG_ID")
	pr, _, err := GetV3Client().PullRequests.Edit(context.TODO(), orgID, envVar("REPO_NAME"), p.PullRequest)
	if err != nil {
		if !strings.Contains(err.Error(), "A pull request already exists") {
			log.Printf("error creating pull request: %v\n", err)
		}
	} else {
		log.Printf("created pull request: %s", pr.GetURL())
	}
}
