job "dispatch" {
    dispatch {
        paused = true
        input_data = "required"
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
                stdin = true
                file = "foo/bar"
            }
        }
    }
}
