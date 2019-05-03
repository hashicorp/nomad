# packngo
Packet Go Api Client

![](https://www.packet.net/media/images/xeiw-packettwitterprofilew.png)


Installation
------------

`go get github.com/packethost/packngo`

Usage
-----

To authenticate to the Packet API, you must have your API token exported in env var `PACKET_AUTH_TOKEN`.

This code snippet initializes Packet API client, and lists your Projects:

```go
package main

import (
	"log"

	"github.com/packethost/packngo"
)

func main() {
	c, err := packngo.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	ps, _, err := c.Projects.List(nil)
	if err != nil {
		log.Fatal(err)
	}
	for _, p := range ps {
		log.Println(p.ID, p.Name)
	}
}

```

This lib is used by the official [terraform-provider-packet](https://github.com/terraform-providers/terraform-provider-packet).

You can also learn a lot from the `*_test.go` sources. Almost all out tests touch the Packet API, so you can see how auth, querying and POSTing works. For example [devices_test.go](devices_test.go).


Linked resources in Get\* and List\* functions
----------------------------------------------
Most of the Get and List functions have *GetOptions resp. *ListOptions paramters. If you supply them, you can specify which attributes of resources in the return set can be excluded or included. This is useful for linked resources, e.g members of a project, devices in a project. 

Linked resources usually have only the `Href` attribute populated, allowing you to fetch them in another API call. But if you explicitly `include` the linked resoruce attribute, it will be populated in the result set of the linking resource.

For example, if you want to list users in a project, you can fetch the project via `Projects.Get(pid, nil)` call. Result from the call will be a Project struct which has `Users []User` attribute. The items in the `[]User` slice only have the URL attribute non-zero, the rest of the fields will be type defaults. You can then parse the ID of the User resources and fetch them consequently. Or, you can use the ListOptions struct in the project fetch call to include the Users (`members` JSON tag) as 

```go
Projects.Get(pid, &packngo.ListOptions{Includes: []{'members'}})` 
```

Then, every item in the `[]User` slice will have all (not only the URL) attributes populated. Following code illustrates the Includes and Excludes.



```go
import (
	"log"

	"github.com/packethost/packngo"
)

func listProjectsAndUsers(lo *packngo.ListOptions) {
	c, err := packngo.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	ps, _, err := c.Projects.List(lo)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Listing for listOptions %+v\n", lo)
	for _, p := range ps {
		log.Printf("project resource %s has %d users", p.Name, len(p.Users))
		for _, u := range p.Users {
			if u.Email != "" && u.FullName != "" {
				log.Printf("  user %s has email %s\n", u.FullName, u.Email)
			} else {
				log.Printf("  only got user link %s\n", u.URL)
			}
		}
	}
}

func main() {
	loMembers := &packngo.ListOptions{Includes: []string{"members"}}
	loMembersOut := &packngo.ListOptions{Excludes: []string{"members"}}
	listProjectsAndUsers(loMembers)
	listProjectsAndUsers(nil)
	listProjectsAndUsers(loMembersOut)
}
```


Acceptance Tests
----------------

If you want to run tests against the actual Packet API, you must set envvar `PACKET_TEST_ACTUAL_API` to non-empty string for the `go test`. The device tests wait for the device creation, so it's best to run a few in parallel.

To run a particular test, you can do

```
$ PACKNGO_TEST_ACTUAL_API=1 go test -v -run=TestAccDeviceBasic
```

If you want to see HTTP requests, set the `PACKNGO_DEBUG` env var to non-empty string, for example:

```
$ PACKNGO_DEBUG=1 PACKNGO_TEST_ACTUAL_API=1 go test -v -run=TestAccVolumeUpdate
```


Committing
----------

Before committing, it's a good idea to run `gofmt -w *.go`. ([gofmt](https://golang.org/cmd/gofmt/))
