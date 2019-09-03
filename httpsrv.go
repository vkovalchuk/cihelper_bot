package main

import (
	"fmt"
	"log"
	"net/http"
)

/*
	Exposed are 2 endpoints:
	POST /pushBBwebhook: handles new BB push, ignore dev and release (will be handled by Jenkins), prompt for product-build job

	GET /buildbot?op=disableBuild&AN_BUILD=an_build: Delete prompt message for this build
*/

func SubscribeEndpoints(b *Bot) {
	http.HandleFunc("/pushBBwebhook", b.handleBitbucketHook)
	http.HandleFunc("/buildbot", b.handleBuildbotOps)
}

func ListenHTTP() {
	fmt.Println("Listening HTTP on :8000")
	err := http.ListenAndServe(":8000", nil)
	fmt.Println("After ListenAndServe, err =", err)
}

func (b *Bot) handleBitbucketHook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Printf("[ERROR] Not implemented method: %s", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	commit_msg := r.PostFormValue("message")
	if len(commit_msg) == 0 {
		w.WriteHeader(http.StatusNoContent)
		w.Write([]byte("could not find commit 'message' parameter in request body"))
		return
	}
	user_name := r.FormValue("user_name")
	user_email := r.FormValue("user_email")
	refChange_refId := r.FormValue("refChange_refId")

	fmt.Println("user_name", user_name, "user_email", user_email, "refChange_refId", refChange_refId)

	refChange_toHash := r.FormValue("refChange_toHash")
	refChange_type := r.FormValue("refChange_type")
	refChange_ts := r.FormValue("refChange_ts")
        // TODO: IMPLEMENT Jenkins trigger
	fmt.Println("refChange_toHash", refChange_toHash, "refChange_type", refChange_type, "refChange_ts", refChange_ts)

	w.Write([]byte(`{"status": "ok"}`))
}

// GET /buildbot?op=disableBuild&AN_BUILD=an_build: Delete prompt message for this build
func (b *Bot) handleBuildbotOps(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.Printf("[ERROR] Not implemented method: %s", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	op := r.FormValue("op")
	if op == "disableBuild" {
		an_build := r.FormValue("AN_BUILD")
		fmt.Println("DISABLE BUILD:", an_build)
		err := b.disableBuild(an_build)
		if err == nil {
			w.Write([]byte("Disabled " + an_build + " successfully."))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("FAILED to disable " + an_build + ": " + err.Error()))
		}

	} else {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Only 'op=disableBuild' is supported"))
	}
}
