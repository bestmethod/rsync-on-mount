# launchd for notifications to go app
```
launchctl load rsync-on-mount.plist
launchctl start com.bestmethod.rsync-on-mount
launchctl stop com.bestmethod.rsync-on-mount
launchctl unload rsync-on-mount.plist
```

## note, you still must run the go app!!!

# code - modify these to your taste
```go
var sourceDir = "testSource/" // will be passed to rsync as source, preferably add trailing / for sanity

var destinationDir = "testDestination/" // will be passed to rsync as destination, add trailing / for sanity

var volumeName = "External" // the name of the volume, as it appears inside the Volume directory

var rsyncDelete = false // should we pass --delete to rsync, depends if you want to sync deletes

var rsyncSwitches = "-avn" // do only -a if you don't want verbose. Since we are local, no need for compression in transit. n == dry run
```

# compile
```
go build rsync-on-mount.go
```

# run
Run however you want, but it MUST be able to access the /private/tmp/volumes-changed, the sourceDir and the destinationDir. It also must be able to state directories in /Volumes (so owner of the mount - i.e. user currently logged in, or root).

Advisable to run as service from launchd or somehow like that, so that this also gets the output logs monitored for your own needs. How yo end up running the script is your deal :)

The launchd script provided is for notifications, not for running the application.
