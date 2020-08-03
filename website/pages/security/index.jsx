import Head from 'next/head'
import HashiHead from '@hashicorp/react-head'
import Content from '@hashicorp/react-content'

export default function ResourcesPage() {
  return (
    <>
      <HashiHead
        is={Head}
        title="Security | Nomad by HashiCorp"
        description="Nomad takes security very seriously. Please responsibly disclose any security vulnerabilities found and we'll handle it quickly."
      />
      <div id="p-security" className="g-grid-container">
        <Content
          product="nomad"
          content={
            <>
              <h2>Nomad Security</h2>

              <p>
                We understand that many users place a high level of trust in
                HashiCorp and the tools we build. We apply best practices and
                focus on security to make sure we can maintain the trust of the
                community.
              </p>

              <p>
                We deeply appreciate any effort to disclose vulnerabilities
                responsibly.
              </p>

              <p>
                If you would like to report a vulnerability, please see the{' '}
                <a href="https://www.hashicorp.com/security">
                  HashiCorp security page
                </a>
                , which has the proper email to communicate with as well as our
                PGP key. Please{' '}
                <strong>
                  do not create an GitHub issue for security concerns
                </strong>
                .
              </p>

              <p>
                If you are not reporting a security sensitive vulnerability,
                please open an issue on the{' '}
                <a href="https://github.com/hashicorp/nomad">
                  Nomad GitHub repository
                </a>
                .
              </p>
            </>
          }
        />
      </div>
    </>
  )
}
