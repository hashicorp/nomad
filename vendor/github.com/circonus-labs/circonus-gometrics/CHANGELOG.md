# v2.2.1

* fix: if submission url host is 'api.circonus.com' do not use private CA in TLSConfig

# v2.2.0

* fix: do not reset counter|gauge|text funcs after each snapshot (only on explicit call to Reset)
* upd: dashboards - optional widget attributes - which are structs - should be pointers for correct omission in json sent to api
* fix: dashboards - remove `omitempty` from required attributes
* fix: graphs - remove `omitempty` from required attributes
* fix: worksheets - correct attribute name, remove `omitempty` from required attributes
* fix: handle case where a broker has no external host or ip set

# v2.1.2

* upd: breaking change in upstream repo
* upd: upstream deps

# v2.1.1

* dep dependencies
* fix two instances of shadowed variables
* fix several documentation typos
* simplify (gofmt -s)
* remove an inefficient use of regexp.MatchString

# v2.1.0

* Add unix socket capability for SubmissionURL `http+unix://...`
* Add `RecordCountForValue` function to histograms

# v2.0.0

* gauges as `interface{}`
   * change: `GeTestGauge(string) (string,error)` ->  `GeTestGauge(string) (interface{},error)`
   * add: `AddGauge(string, interface{})` to add a delta value to an existing gauge
* prom output candidate
* Add `CHANGELOG.md` to repository
