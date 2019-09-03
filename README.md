# cihelper_bot

## Scenario

We had particular software development process:
* Some Jenkins instance creates new builds in different branches.
* These builds sometimes need LONG (hours) integration testing, sometimes not.
** Tests run on cluster and cannot run in parallel.
* The user who started particular build should decide that.
* This is *Slack bot* that helps users to start tests (also separate Jenkins jobs).

## Logic

When new build is SUCCESSFUL Jenkins sends "New Build created" Slack message to the user who started the build.
The message should contain "Run tests" link/button to run test Jenkins job for this specific build.
Also there should be "Skip tests" button in the message. It removes "New Build created" message.

Bot should track: 
* some other user's tests are running so new tests cannot be run right now; 
* when a user started tests replace "Run tests" and "Skip tests" buttons with "Tests are in progress" and URL; 
* when test job finishes, update its message with test result, restores all other pending messages.

## Build

go build

## Custom logic to adapt

* rules.go: The list of patterns for Slack messages, parsing logic and reactions;
* httpsrv.go: This Slack bot is combined with HTTP server. Just for convenience.
