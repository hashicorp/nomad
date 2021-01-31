import Head from 'next/head'
import HashiHead from '@hashicorp/react-head'
import Content from '@hashicorp/react-content'

export default function ResourcesPage() {
  return (
    <>
      <HashiHead
        is={Head}
        title="Community | Nomad by HashiCorp"
        description="Nomad is widely deployed across a range of enterprises and business verticals."
      />
      <div id="p-resources" className="g-grid-container">
        <Content
          product="nomad"
          content={
            <>
              <h2>Community</h2>
              <p>
                Nomad is widely adopted and used in production by organizations
                like Cloudflare, Roblox, Pandora, PagerDuty, and more.
              </p>
              <p>
                This is a collection of resources for joining our community and
                learning Nomad&apos;s real world use-cases.
              </p>
              <p>
                <strong>Discussion Forum</strong>
                <ul>
                  <li>
                    <a href="https://discuss.hashicorp.com/c/nomad">
                      HashiCorp Discuss
                    </a>
                  </li>
                </ul>
              </p>
              <p>
                <strong>Virtual Talks</strong>
                <ul>
                  <li>
                    <a href="https://www.hashicorp.com/resources/nomad-scheduling-101-why-let-container-runtimes-have-all-the-fun/">
                      6/9 Scheduling Containers, .NET, Java Applications with
                      Nomad (2020)
                    </a>
                  </li>
                  <li>
                    <a href="https://www.hashicorp.com/resources/nomad-ci-cd-developer-workflows-and-integrations/">
                      5/29 Build a CI/CD Pipeline with Nomad & Gitlab (2020)
                    </a>
                  </li>
                  <li>
                    <a href="https://www.hashicorp.com/resources/cloud-bursting-made-real-with-nomad-and-consul/">
                      5/29 Cloud Bursting Demo with Nomad & Consul (2020)
                    </a>
                  </li>
                  <li>
                    <a href="https://www.hashicorp.com/resources/nomad-expert-panel-live-q2-2020/">
                      5/29 Nomad Expert Panel Live Q&A (2020)
                    </a>
                  </li>
                  <li>
                    <a href="https://www.hashicorp.com/resources/governance-for-multiple-teams-sharing-a-nomad-cluster/">
                      5/15 Governance for Multiple Teams Sharing a Nomad Cluster
                      (2020)
                    </a>
                  </li>
                  <li>
                    <a href="https://www.hashicorp.com/resources/modern-scheduling-for-modern-applications-with-nomad/">
                      {' '}
                      4/20 Modern Scheduling for Modern Applications with Nomad
                      (2020)
                    </a>
                  </li>
                  <li>
                    <a href="https://www.hashicorp.com/resources/solutions-engineering-hangout-integrating-nomad-with-vault/">
                      {' '}
                      4/17 Integrating Nomad with Vault Demo (2020)
                    </a>
                  </li>
                  <li>
                    <a href="https://www.hashicorp.com/resources/nomad-tech-deep-dive-autoscaling-csi-plugins-and-more/">
                      4/14 Nomad Deep Dive: Autoscaling, CSI Plugins, and More
                      (2020)
                    </a>
                  </li>
                  <li>
                    <a href="https://www.hashicorp.com/resources/nomad-virtual-day-2020-panel-discussion/">
                      3/16 Nomad Panel Discussion with Roblox (2020)
                    </a>
                  </li>
                  <li>
                    <a href="https://www.hashicorp.com/resources/hashicorp-nomad-vs-kubernetes-comparing-complexity/">
                      3/3 Nomad vs. Kubernetes - Comparing Complexity (2020)
                    </a>
                  </li>
                  <li>
                    <a href="https://www.hashicorp.com/resources/ground-control-to-nomad-job-dispatch/">
                      2/27 Ground Control to Nomad Job Dispatch (2020)
                    </a>
                  </li>
                  <li>
                    <a href="https://www.hashicorp.com/resources/nomad-best-practices-for-reliable-deploys/">
                      2/27 Best Practices for Reliable Deploys on Nomad (2020)
                    </a>
                  </li>
                  <li>
                    <a href="https://www.hashicorp.com/resources/monitoring-nomad-with-prometheus-and-icinga/">
                      2/27 Monitoring Nomad with Prometheus and Incinga (2020)
                    </a>
                  </li>
                </ul>
              </p>
              <p>
                <strong>Bug Tracker</strong>
                <ul>
                  <li>
                    <a href="https://github.com/hashicorp/nomad/issues">
                      GitHub Issues
                    </a>
                  </li>
                </ul>
                Please only use this to report bugs. For general help, please
                use our{' '}
                <a href="https://discuss.hashicorp.com/c/nomad">
                  discussion forum
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
