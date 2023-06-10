package dsync

import (
	"log"
	"os"
	"regexp"
)

func ErrChk(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func DirExist(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return true
}

func Contains(strRange []string, pattern string) bool {
	for _, val := range strRange {
		match, _ := regexp.MatchString(pattern, val)
		return match
	}

	return false
}

func Filter(arr []string, cond func(string) bool) []string {
	result := []string{}
	for i := range arr {
		if cond(arr[i]) {
			result = append(result, arr[i])
		}
	}
	return result
}
