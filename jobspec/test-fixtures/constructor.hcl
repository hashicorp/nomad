job "constructor" {
    constructor {
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

            dispatch_input {
                file = "foo/bar"
            }
        }
    }
}
