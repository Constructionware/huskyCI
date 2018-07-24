package analysis

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/globocom/husky/types"
	"gopkg.in/mgo.v2/bson"
)

// EnryStartAnalysis checks the languages of a repository, update them into mongoDB, and starts corresponding new securityTests.
func EnryStartAnalysis(CID string, cleanedOutput string, RID string) {

	// step 0: get analysis based on RID
	analysisQuery := map[string]interface{}{"containers.CID": CID}
	analysis, err := FindOneDBAnalysis(analysisQuery)
	if err != nil {
		fmt.Println("Could not find analysis by this RID:", err)
		return
	}

	// step 1: get each language found in cOutput
	mapLanguages := make(map[string][]interface{})
	err = json.Unmarshal([]byte(cleanedOutput), &mapLanguages)
	if err != nil {
		fmt.Println("Unmarshall error:", err)
		return
	}
	repositoryLanguages := []types.Language{}
	newLanguage := types.Language{}
	for name, files := range mapLanguages {
		fs := []string{}
		for _, f := range files {
			if reflect.TypeOf(f).String() == "string" {
				fs = append(fs, f.(string))
			} else {
				fmt.Println("Error mapping languages.")
				return
			}
		}
		newLanguage = types.Language{
			Name:  name,
			Files: fs,
		}
		repositoryLanguages = append(repositoryLanguages, newLanguage)
	}

	// step 2: update repository with the languages found and with each corresponding default securityTests
	newSecurityTests := []types.SecurityTest{}
	// inserting generic securityTests first.
	genericSecurityTestQuery := map[string]interface{}{"language": "Generic", "default": true}
	genericSecurityTestResult, err := FindAllDBSecurityTest(genericSecurityTestQuery)
	if err != nil {
		fmt.Println("Error finding default generic securityTest:", err)
		return
	}
	for _, genericSecurityTest := range genericSecurityTestResult {
		newSecurityTests = append(newSecurityTests, genericSecurityTest)
	}
	// inserting new securityTests based on the languages found.
	for _, language := range repositoryLanguages {
		languageSecurityTestQuery := map[string]interface{}{"language": language.Name, "default": true}
		languageSecurityTestResult, err := FindOneDBSecurityTest(languageSecurityTestQuery)
		if err == nil {
			newSecurityTests = append(newSecurityTests, languageSecurityTestResult)
		}
	}
	// updating repository.
	repositoryQuery := map[string]interface{}{"URL": analysis.URL}
	updateRepositoryQuery := bson.M{
		"$set": bson.M{
			"securityTests": newSecurityTests,
			"languages":     repositoryLanguages,
		},
	}
	err = UpdateOneDBRepository(repositoryQuery, updateRepositoryQuery)
	if err != nil {
		fmt.Println("Could not update repository's securityTests:", err)
		return
	}

	// step 3: update analysis with the new securityTests
	analysis.SecurityTests = newSecurityTests
	err = UpdateOneDBAnalysis(analysisQuery, analysis)
	if err != nil {
		fmt.Println("Error updating AnalysisCollection:", err)
	}

	// step 4: start new securityTests
	for _, securityTest := range newSecurityTests {
		if securityTest.Name != "enry" {
			go DockerRun(RID, &analysis, securityTest)
		}
	}
}
