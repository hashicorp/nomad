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
                use our mailing list or Gitter.
              </p>
              <h3>Who Uses Nomad</h3>
              <h4>Roblox</h4>
              <ul>
                <li>
                  <a href="https://www.hashicorp.com/case-studies/roblox/">
                    How Roblox built a platform for 100 million players on Nomad
                    (2020)
                  </a>
                </li>
                <li>
                  <a href="https://portworx.com/architects-corner-roblox-runs-platform-70-million-gamers-hashicorp-nomad/">
                    How Roblox built a platform for 70 million gamers on Nomad
                    (2019)
                  </a>
                </li>
              </ul>
              <h4>Cloudflare</h4>
              <ul>
                <li>
                  <a href="https://blog.cloudflare.com/how-we-use-hashicorp-nomad/">
                    How Cloudflare Uses HashiCorp Nomad (2020)
                  </a>
                </li>
              </ul>
              <h4>BetterHelp</h4>
              <ul>
                <li>
                  <a href="https://www.youtube.com/watch?v=eN2ghrGpiUo">
                    How the world's largest online therapy provider runs on
                    Nomad (2020)
                  </a>
                </li>
              </ul>
              <h4>Navi Capital</h4>
              <ul>
                <li>
                  <a href="https://www.hashicorp.com/blog/nomad-community-story-navi-capital/">
                    How Nomad powers a $1B hedge fund in Brazil (2020)
                  </a>
                </li>
              </ul>
              <h4>Trivago</h4>
              <ul>
                <li>
                  <a href="https://endler.dev/2019/maybe-you-dont-need-kubernetes/">
                    Maybe You Don't Need Kubernetes (2019)
                  </a>
                </li>
                <li>
                  <a href="https://tech.trivago.com/2019/01/25/nomad-our-experiences-and-best-practices/">
                    Nomad - Our Experiences and Best Practices (2019)
                  </a>
                </li>
              </ul>
              <h4>Reaktor</h4>
              <ul>
                <li>
                  <a href="https://youtu.be/GkmyNBUugg8">
                    Nomad: Kubernetes, but without the complexity (2019)
                  </a>
                </li>
              </ul>
              <h4>Pandora</h4>
              <ul>
                <li>
                  <a href="https://www.youtube.com/watch?v=OsZeKTP2u98&t=2s">
                    How Pandora Uses Nomad (2019)
                  </a>
                </li>
              </ul>
              <h4>CircleCI</h4>
              <ul>
                <li>
                  <a href="https://stackshare.io/circleci/how-circleci-processes-4-5-million-builds-per-month">
                    {' '}
                    How CircleCI Processes 4.5 Million Builds Per Month with
                    Nomad (2019)
                  </a>
                </li>
                <li>
                  <a href="https://www.hashicorp.com/resources/nomad-vault-circleci-security-scheduling">
                    {' '}
                    Security & Scheduling Are Not Your Core Competencies (2018)
                  </a>
                </li>
              </ul>
              <h4>Q2</h4>
              <ul>
                <li>
                  <a href="https://www.youtube.com/watch?v=OsZeKTP2u98&feature=youtu.be&t=1499">
                    {' '}
                    Q2's Nomad Use and Overview (2019)
                  </a>
                </li>
              </ul>
              <h4>Deluxe Entertainment</h4>
              <ul>
                <li>
                  <a href="https://www.hashicorp.com/resources/deluxe-hashistack-video-production">
                    {' '}
                    How We Use the HashiStack for Video Production (2018)
                  </a>
                </li>
              </ul>
              <h4>SAP</h4>
              <ul>
                <li>
                  <a href="https://www.hashicorp.com/resources/nomad-community-call-core-team-sap-ariba">
                    How We Use Nomad @ SAP Ariba (2018)
                  </a>
                </li>
              </ul>
              <h4>PagerDuty</h4>
              <ul>
                <li>
                  <a href="https://www.hashicorp.com/resources/pagerduty-nomad-journey">
                    PagerDuty's Nomadic Journey (2017)
                  </a>
                </li>
              </ul>
              <h4>Target</h4>
              <ul>
                <li>
                  <a href="https://www.hashicorp.com/resources/nomad-scaling-target-microservices-across-cloud">
                    {' '}
                    Nomad at Target: Scaling Microservices Across Public and
                    Private Clouds (2018)
                  </a>
                </li>
                <li>
                  <a href="https://danielparker.me/nomad/hashicorp/schedulers/nomad/">
                    {' '}
                    Playing with Nomad from HashiCorp (2017)
                  </a>
                </li>
              </ul>
              <h4>Oscar Health</h4>
              <ul>
                <li>
                  <a href="https://www.hashicorp.com/resources/scalable-ci-oscar-health-insurance-nomad-docker">
                    Scalable CI at Oscar Health with Nomad and Docker (2018)
                  </a>
                </li>
              </ul>
              <h4>eBay</h4>
              <ul>
                <li>
                  <a href="https://www.hashicorp.com/resources/ebay-hashistack-fully-containerized-platform-iac">
                    HashiStack at eBay: A Fully Containerized Platform Based on
                    Infrastructure as Code (2018)
                  </a>
                </li>
              </ul>
              <h4>Dutch National Police</h4>
              <ul>
                <li>
                  <a href="https://www.hashicorp.com/resources/going-cloud-native-at-the-dutch-national-police">
                    Going Cloud-Native at the Dutch National Police (2018)
                  </a>
                </li>
              </ul>
              <h4>N26</h4>
              <ul>
                <li>
                  <a href="https://medium.com/insiden26/tech-at-n26-the-bank-in-the-cloud-e5ff818b528b">
                    Tech at N26 - The Bank in the Cloud (2018)
                  </a>
                </li>
              </ul>
              <h4>NIH NCBI</h4>
              <ul>
                <li>
                  <a href="https://www.hashicorp.com/resources/ncbi-legacy-migration-hybrid-cloud-consul-nomad">
                    NCBI's Legacy Migration to Hybrid Cloud with Nomad & Consul
                    (2018)
                  </a>
                </li>
              </ul>
              <h4>Citadel</h4>
              <ul>
                <li>
                  <a href="https://www.hashicorp.com/resources/end-to-end-production-nomad-citadel">
                    End-to-End Production Nomad at Citadel (2017)
                  </a>
                </li>
                <li>
                  <a href="https://www.hashicorp.com/resources/citadel-scaling-hashicorp-nomad-consul">
                    Extreme Scaling with HashiCorp Nomoad & Consul (2016)
                  </a>
                </li>
              </ul>
              <h4>Jet.com</h4>
              <ul>
                <li>
                  <a href="https://www.hashicorp.com/resources/jet-walmart-hashicorp-nomad-azure-run-apps">
                    Driving down costs at Jet.com with HashiCorp Nomad (2017)
                  </a>
                </li>
              </ul>
              <h4>Elsevier</h4>
              <ul>
                <li>
                  <a href="https://www.hashicorp.com/resources/elsevier-nomad-container-framework-demo">
                    Elsevier's Container Framework with Nomad, Terraform, and
                    Consul (2017)
                  </a>
                </li>
              </ul>
              <h4>Graymeta</h4>
              <ul>
                <li>
                  <a href="https://www.hashicorp.com/resources/backend-batch-processing-nomad">
                    Backend Batch Process at Scale with Nomad (2017)
                  </a>
                </li>
              </ul>
              <h4>imigx</h4>
              <ul>
                <li>
                  <a href="https://medium.com/@copyconstruct/schedulers-kubernetes-and-nomad-b0f2e14a896">
                    Cluster Schedulers & Why We Chose Nomad over Kubernetes
                    (2017)
                  </a>
                </li>
              </ul>
            </>
          }
        />
      </div>
    </>
  )
}
