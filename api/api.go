package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/camptocamp/terraboard/util"
)

var svc *s3.S3
var bucket string
var states []string
var stateVersions map[string]*StateVersions

func init() {
	sess := session.Must(session.NewSession())
	svc = s3.New(sess, &aws.Config{})
	bucket = os.Getenv("AWS_BUCKET")
	stateVersions = make(map[string]*StateVersions)

	//buildCache()
}

func buildCache() {
	log.Infof("Building initial cache")

	err := refreshStates()
	if err != nil {
		log.Errorf("Failed to build cache: %s", err)
	}

	for _, st := range states {
		GetState(st, "")

		versions, _ := getVersions(st)
		for _, v := range versions {
			GetState(st, *v.VersionId)
		}
	}
}

func refreshStates() error {
	result, err := svc.ListObjects(&s3.ListObjectsInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return err
	}

	var keys []string
	for _, obj := range result.Contents {
		if strings.HasSuffix(*obj.Key, ".tfstate") {
			keys = append(keys, *obj.Key)
		}
	}
	states = keys
	return nil
}

func ApiStates(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	err := refreshStates()
	if err != nil {
		errObj := make(map[string]string)
		errObj["error"] = "Failed to list states"
		errObj["details"] = fmt.Sprintf("%v", err)
		j, _ := json.Marshal(errObj)
		io.WriteString(w, string(j))
		return
	}

	j, _ := json.Marshal(states)
	io.WriteString(w, string(j))
}

func ApiState(w http.ResponseWriter, r *http.Request) {
	st := util.TrimBase(r, "api/state")
	versionId := r.URL.Query().Get("versionid")
	state, err := GetState(st, versionId)
	if err != nil {
		io.WriteString(w, fmt.Sprintf("%s", err))
		return
	}

	jState, _ := json.Marshal(state)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	io.WriteString(w, string(jState))
}

func getVersions(prefix string) (versions []*s3.ObjectVersion, err error) {
	result, err := svc.ListObjectVersions(&s3.ListObjectVersionsInput{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return versions, err
	}
	return result.Versions, nil
}

func ApiHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	st := util.TrimBase(r, "api/history/")
	result, err := getVersions(st)
	if err != nil {
		errObj := make(map[string]string)
		errObj["error"] = fmt.Sprintf("State file history not found: %v", st)
		errObj["details"] = fmt.Sprintf("%v", err)
		j, err := json.Marshal(errObj)
		if err != nil {
			log.Errorf("Failed to marshal json: %v", err)
		}
		io.WriteString(w, string(j))
		return
	}

	j, err := json.Marshal(result)
	if err != nil {
		log.Errorf("Failed to marshal json: %v", err)
	}
	io.WriteString(w, string(j))
}
