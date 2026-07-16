package main

import "testing"

func TestSummonerProfilesDoesNotRequirePatchArgument(t *testing.T) {
	if patchRequired("summoner-profiles") {
		t.Fatal("summoner profile maintenance must be resumable without a patch argument")
	}
}
