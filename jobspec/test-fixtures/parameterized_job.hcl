job "parameterized_job" {
    parameterized {
        payload = "required"
        meta_keys {
            required = ["foo", "bar"]
            optional = ["baz", "bam"]
        }
    }
    group "foo" {
        task "bar" {
            driver = "docker"
            resources {}

            dispatch_payload {
                file = "foo/bar"
            }
        }
    }
}
