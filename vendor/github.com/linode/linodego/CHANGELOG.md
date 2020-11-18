# Change Log

## [v0.10.0](https://github.com/linode/linodego/compare/v0.9.2..v0.10.0) (2019-06-25)

### Breaking Changes

* Change to `AllowAutoDiskResize` to `*bool` is a breaking change, so bump minor version.

## [v0.9.2](https://github.com/linode/linodego/compare/v0.9.1..v0.9.2) (2019-06-25)

### Breaking Changes

* `AllowAutoDiskResize` on `InstanceResizeOptions` is now a pointer, allowing it to be set to false.

### Features

* Support the `PersistAcrossBoots` flag, allowing users to set it to false and attach more disks.

## [v0.9.1](https://github.com/linode/linodego/compare/v0.9.0..v0.9.1) (2019-06-18)

### Fixes

* Fix the json struct tag for the AllowAutoDiskResize flag on Linodes

### Features

* Support alternative root CA certificates with `Client.SetRooteCertificate`
* Support setting a client's API token with `Client.SetToken`

## [v0.9.0](https://github.com/linode/linodego/compare/v0.8.1..v0.9.0) (2019-05-24)

### Breaking Changes

* `ResizeInstance` now takes `ResizeInstanceOptions` to support `AllowAutoDiskResize` (API v4.0.23)
* `ResizeInstanceOptions` renamed to `InstanceResizeOptions` to fit convention
* `RescueInstanceOptions` renamed to `InstanceRescueOptions` to fit convention

### Features

* Adds new `EventAction` constants: `ActionLinodeMutateCreate`, `ActionLinodeResizeCreate`, `ActionLishBoot` (API v4.0.23)
* Adds `GetInstanceTransfer` which returns an `InstanceTransfer` (API v4.0.23)

## [v0.8.1](https://github.com/linode/linodego/compare/v0.8.0..v0.8.1) (2019-05-20)

### Features

* add `LINODE_URL` environment variable

## [v0.8.0](https://github.com/linode/linodego/compare/v0.7.1..v0.8.0) (2019-05-01)

### Fixes

* *breaking change* const `ActionTicketReply` is now `ActionTicketUpdate`
* *breaking change* `InstanceIP` `Type` values are now represented by `InstanceIPType` string constants

### Features

* optimized `WaitForEventFinished` event polling (and deduped its logs)
* add `GetInstanceStats` and `GetInstanceStatsByDate`
* add `UpdateInstanceIPAddress`
* add `GetAccountSettings` and `UpdateAccountSettings`
* add `CreatePayment` `GetPayment` `ListPayments`
* add `CreateOAuthClient`, `ListOAuthClients`, `GetOAuthClient`, `UpdateOAuthClient`, `DeleteOAuthClient`
* added many new `EventAction` constants `ActionAccountUpdate`, `ActionAccountSettingsUpdate`, `ActionCommunityLike`, `ActionDiskUpdate`, `ActionDNSRecordUpdate`, `ActionDNSZoneUpdate`, `ActionHostReboot`, `ActionImageUpdate`, `ActionLassieReboot`, `ActionLinodeUpdate`, `ActionLinodeConfigCreate`, `ActionLinodeConfigDelete`, `ActionLinodeConfigUpdate`, `ActionLongviewClientUpdate`, `ActionNodebalancerUpdate`, `ActionNodebalancerConfigUpdate`, `ActionStackScriptUpdate`, `ActionVolumeUpdate` (API v4.0.17)
* added `EntityType` constants `EntityDisk`, `EntityDomain`, `EntityNodebalancer` (the Linode API now permits these in ListEvents Filters keyed with `event.id` and `event.type`)
* added `ActionCommunityLike` `EventAction` constant (API v4.0.11)
* added `IPv6Range` `Prefix` (API v4.0.11, Only populated for the regional floating pools (`/116`), not the Instance bound ranges (`/64`, `/56`).  See [Additional IPv6 Addresses](https://www.linode.com/docs/networking/an-overview-of-ipv6-on-linode/#additional-ipv6-addresses))
* added `LogoURL` and `Ordinal` to `Stackscript` (API v4.0.20)
* added `Reserved` to `InstanceIPv4Response` (API v4.0.20, when present, indicates IP addresses that will be available after a cross region migration)
* switched from `metalinter` to `golangci-lint`
* switched to `go mod` from `dep`

<a name="v0.7.1"></a>

## [v0.7.1](https://github.com/linode/linodego/compare/v0.7.0..v0.7.1) (2019-02-05)

### Features

* add `ClassDedicated` constant (`dedicated`) for use in `LinodeType` `Class` values
  See the [Dedicated CPU Announcement](https://blog.linode.com/2019/02/05/introducing-linode-dedicated-cpu-instances/)

<a name="v0.7.0"></a>

## [v0.7.0](https://github.com/linode/linodego/compare/v0.6.2..v0.7.0) (2018-12-03)

### Features

* add `Tags` field in: `NodeBalancer`, `Domain`, `Volume`
* add `UpdateIPAddress` (for setting RDNS)

### Fixes

* invalid URL for `/v4/networking/` enpoints (IPv6 Ranges and Pools) has been correcrted

<a name="v0.6.2"></a>

## [v0.6.2](https://github.com/linode/linodego/compare/v0.6.1..v0.6.2) (2018-10-26)

### Fixes

* add missing `Account` fields: `address_1`, `address_2`, `phone`

<a name="v0.6.1"></a>
## [v0.6.1](https://github.com/linode/linodego/compare/v0.6.0..v0.6.1) (2018-10-26)

### Features

* Adds support for fetching and updating basic Profile information

<a name="v0.6.0"></a>
## [v0.6.0](https://github.com/linode/linodego/compare/v0.5.1..v0.6.0) (2018-10-25)

### Fixes

* Fixes Image date handling
* Fixes broken example code in README
* Fixes WaitForEventFinished when encountering events without entity
* Fixes ResizeInstanceDisk which was executing CloneInstanceDisk
* Fixes go-resty import path to gopkg.in version for future go module support

### Features

* Adds support for user account operations
* Adds support for profile tokens
* Adds support for Tags
* Adds PasswordResetInstanceDisk
* Adds DiskStatus constants
* Adds WaitForInstanceDiskStatus
* Adds SetPollDelay for configuring poll duration

  * Reduced polling time to millisecond granularity
  * Change polling default to 3s to avoid 429 conditions
  * Use poll delay in waitfor functions

<a name="v0.5.1"></a>
## [v0.5.1](https://github.com/linode/linodego/compare/v0.5.0...v0.5.1) (2018-09-10)

### Fixes

* Domain.Status was not imported from API responses correctly

<a name="v0.5.0"></a>
## [v0.5.0](https://github.com/linode/linodego/compare/v0.4.0...v0.5.0) (2018-09-09)

### Breaking Changes

* List functions return slice of thing instead of slice of pointer to thing

### Feature

* add SSHKeys methods to client (also affects InstanceCreate, InstanceDiskCreate)
* add RebuildNodeBalancerConfig (and CreateNodeBalancerConfig with Nodes)

### Fixes

* Event.TimeRemaining wouldn't parse all possible API value
* Tests no longer rely on known/special instance and volume ids

<a name="0.4.0"></a>
## [0.4.0](https://github.com/linode/linodego/compare/v0.3.0...0.4.0) (2018-08-27)

### Breaking Changes

Replaces bool, error results with error results, for:

* instance\_snapshots.go: EnableInstanceBackups
* instance\_snapshots.go: CancelInstanceBackups
* instance\_snapshots.go: RestoreInstanceBackup
* instances.go: BootInstance
* instances.go: RebootInstance
* instances.go: MutateInstance
* instances.go: RescueInstance
* instances.go: ResizeInstance
* instances.go: ShutdownInstance
* volumes.go: DetachVolume
* volumes.go: ResizeVolume


### Docs

* reword text about breaking changes until first tag

### Feat

* added MigrateInstance and InstanceResizing from 4.0.1-4.0.3 API Changelog
* added gometalinter to travis builds
* added missing function and type comments as reported by linting tools
* supply json values for all fields, useful for mocking responses using linodego types
* use context channels in WaitFor\* functions
* add LinodeTypeClass type (enum)
* add TicketStatus type (enum)
* update template thing and add a test template

### Fix

* TransferQuota was TransferQuote (and not parsed from the api correctly)
* stackscripts udf was not parsed correctly
* add InstanceCreateOptions.PrivateIP
* check the WaitFor timeout before sleeping to avoid extra sleep
* various linting warnings and unhandled err results as reported by linting tools
* fix GetStackscript 404 handling


<a name="0.3.0"></a>

## [0.3.0](https://github.com/linode/linodego/compare/v0.2.0...0.3.0) (2018-08-15)

### Breaking Changes

* WaitForVolumeLinodeID return fetch volume for consistency with out WaitFors
* Moved linodego from chiefy to github.com/linode. Thanks [@chiefy](https://github.com/chiefy)!

<a name="v0.2.0"></a>

## [v0.2.0](https://github.com/linode/linodego/compare/v0.1.1...v0.2.0) (2018-08-11)

### Breaking Changes

* WaitFor\* should be client methods
  *use `client.WaitFor...` rather than `linodego.WaitFor(..., client, ...)`*

* remove ListInstanceSnapshots (does not exist in the API)
  *this never worked, so shouldn't cause a problem*

* Changes UpdateOptions and CreateOptions and similar Options parameters to values instead of pointers
  *these were never optional and the function never updated any values in the Options structures*

* fixed various optional/zero Update and Create options
  *some values are now pointers, and vice-versa*

  * Changes InstanceUpdateOptions to use pointers for optional fields Backups and Alerts
  * Changes InstanceClone's Disks and Configs to ints instead of strings

* using new enum string aliased types where appropriate
  *`InstanceSnapshotStatus`, `DiskFilesystem`, `NodeMode`*

### Feature

* add RescueInstance and RescueInstanceOptions
* add CreateImage, UpdateImage, DeleteImage
* add EnableInstanceBackups, CancelInstanceBackups, RestoreInstanceBackup
* add WatchdogEnabled to InstanceUpdateOptions

### Fix

* return Volume from AttachVolume instead of bool
* add more boilerplate to template.go
* nodebalancers and domain records had no pagination support
* NodeBalancer transfer stats are not int

### Tests

* add fixtures and tests for NodeBalancerNodes
* fix nodebalancer tests to handle changes due to random labels
* add tests for nodebalancers and nodebalancer configs
* added tests for Backups flow
* TestListInstanceBackups fixture is hand tweaked because repeated polled events
  appear to get the tests stuck

### Deps

* update all dependencies to latest

<a name="v0.1.1"></a>

## [v0.1.1](https://github.com/linode/linodego/compare/v0.0.1...v0.1.0) (2018-07-30)

Adds more Domain handling

### Fixed

* go-resty doesnt pass errors when content-type is not set
* Domain, DomainRecords, tests and fixtures

### Added

* add CreateDomainRecord, UpdateDomainRecord, and DeleteDomainRecord

<a name="v0.1.0"></a>

## [v0.1.0](https://github.com/linode/linodego/compare/v0.0.1...v0.1.0) (2018-07-23)

Deals with NewClient and context for all http requests

### Breaking Changes

* changed `NewClient(token, *http.RoundTripper)` to `NewClient(*http.Client)`
* changed all `Client` `Get`, `List`, `Create`, `Update`, `Delete`, and `Wait` calls to take context as the first parameter

### Fixed

* fixed docs should now show Examples for more functions

### Added

* added `Client.SetBaseURL(url string)`

<a name="v0.0.1"></a>
## v0.0.1 (2018-07-20)

### Changed

* Initial tagged release
