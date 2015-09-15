module CommandHelpers
  # Returns the markdown text for the general options usage.
  def general_options_usage()
    <<EOF
* `-address=<addr>`: The address of the Nomad server. Overrides the "NOMAD_ADDR"
  environment variable if set. Defaults to "http://127.0.0.1:4646".
EOF
  end
end
