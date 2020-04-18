import Head from 'next/head'
import HashiHead from '@hashicorp/react-head'
import Content from '@hashicorp/react-content'

export default function ResourcesPage() {
  return (
    <>
      <HashiHead
        is={Head}
        title="Resources | Nomad by HashiCorp"
        description="Nomad is widely deployed across a range of enterprises and business verticals."
      />
      <div id="p-resources" className="g-grid-container">
        <Content
          product="nomad"
          content={
            <>
              <h2>Resources</h2>
              <p>
                Nomad is widely adopted and used in production by PagerDuty,
                Target, Citadel, Trivago, SAP, Pandora, Roblox, eBay, Deluxe
                Entertainment, and more.
              </p>
              <p>
                This is a collection of resources for installing Nomad, joining
                our community, learning Nomad&apos;s real world use-cases, or
                integrating with third-party tools.
              </p>
              <h3>Community</h3>
              <p>
                <strong>Discussion Forum</strong>
                <ul>
                  <li>
                    <a href="https://discuss.hashicorp.com/c/nomad">
                      HashiCorp Discussion Forum (Nomad Category)
                    </a>
                  </li>
                </ul>
              </p>
              <p>
                <strong>Mailing List</strong>
                <ul>
                  <li>
                    <a href="https://groups.google.com/group/nomad-tool">
                      Nomad Google Group
                    </a>
                  </li>
                </ul>
              </p>
              <p>
                <strong>Gitter</strong>
                <ul>
                  <li>
                    <a href="https://gitter.im/hashicorp-nomad/Lobby">
                      Nomad Chat Room
                    </a>
                  </li>
                </ul>
              </p>
              <p>
                <strong>Community Calls</strong>
                <ul>
                  <li>
                    <a href="https://www.youtube.com/watch?v=OsZeKTP2u98&t=2s">
                      04/03/2019 with Pandora &amp; Q2EBanking
                    </a>
                  </li>
                  <li>
                    <a href="https://www.youtube.com/watch?v=eSwZwVVTDqw&t=2660s">
                      05/24/2018 with SAP Ariba
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
              <ul>
                <li>CircleCI</li>
                <ul>
                  <li>
                    <a href="https://www.hashicorp.com/resources/nomad-vault-circleci-security-scheduling">
                      Security &amp; Scheduling are Not Your Core Competencies
                    </a>
                  </li>
                </ul>
                <li>Citadel</li>
                <ul>
                  <li>
                    <a href="https://www.hashicorp.com/resources/end-to-end-production-nomad-citadel">
                      End-to-End Production Nomad at Citadel
                    </a>
                  </li>
                  <li>
                    <a href="https://www.hashicorp.com/resources/citadel-scaling-hashicorp-nomad-consul">
                      Extreme Scaling with HashiCorp Nomad &amp; Consul
                    </a>
                  </li>
                </ul>
                <li>Deluxe Entertainment</li>
                <ul>
                  <li>
                    <a href="https://www.hashicorp.com/resources/deluxe-hashistack-video-production">
                      How Deluxe Uses the Complete HashiStack for Video
                      Production
                    </a>
                  </li>
                </ul>
                <li>Jet.com (Walmart)</li>
                <ul>
                  <li>
                    <a href="https://www.hashicorp.com/resources/jet-walmart-hashicorp-nomad-azure-run-apps">
                      Driving down costs at Jet.com with HashiCorp Nomad
                    </a>
                  </li>
                </ul>
                <li>PagerDuty</li>
                <ul>
                  <li>
                    <a href="https://www.hashicorp.com/resources/pagerduty-nomad-journey">
                      PagerDuty&apos;s Nomadic Journey
                    </a>
                  </li>
                </ul>
                <li>Pandora</li>
                <ul>
                  <li>
                    <a href="https://www.youtube.com/watch?v=OsZeKTP2u98&t=2s">
                      How Pandora Uses Nomad
                    </a>
                  </li>
                </ul>
                <li>SAP Ariba</li>
                <ul>
                  <li>
                    <a href="https://www.hashicorp.com/resources/nomad-community-call-core-team-sap-ariba">
                      HashiCorp Nomad @ SAP Ariba
                    </a>
                  </li>
                </ul>
                <li>SeatGeek</li>
                <ul>
                  <li>
                    <a href="https://github.com/seatgeek/nomad-helper">
                      Nomad Helper Tools
                    </a>
                  </li>
                </ul>
                <li>Spaceflight Industries</li>
                <ul>
                  <li>
                    <a href="https://www.hashicorp.com/blog/spaceflight-uses-hashicorp-consul-for-service-discovery-and-real-time-updates-to-their-hub-and-spoke-network-architecture">
                      Spaceflight&apos;s Hub-And-Spoke Infrastructure
                    </a>
                  </li>
                </ul>
                <li>SpotInst</li>
                <ul>
                  <li>
                    <a href="https://www.hashicorp.com/blog/spotinst-and-hashicorp-nomad-to-reduce-ec2-costs-fo">
                      SpotInst and HashiCorp Nomad to Reduce EC2 Costs for Users
                    </a>
                  </li>
                </ul>
                <li>Target</li>
                <ul>
                  <li>
                    <a href="https://www.hashicorp.com/resources/nomad-scaling-target-microservices-across-cloud">
                      Nomad at Target: Scaling Microservices Across Public and
                      Private Cloud
                    </a>
                  </li>
                  <li>
                    <a href="https://danielparker.me/nomad/hashicorp/schedulers/nomad/">
                      Playing with Nomad from HashiCorp
                    </a>
                  </li>
                </ul>
                <li>Trivago</li>
                <ul>
                  <li>
                    <a href="https://matthias-endler.de/2019/maybe-you-dont-need-kubernetes/">
                      Maybe You Don&apos;t Need Kubernetes
                    </a>
                  </li>
                  <li>
                    <a href="https://tech.trivago.com/2019/01/25/nomad-our-experiences-and-best-practices/">
                      Nomad - Our Experiences and Best Practices
                    </a>
                  </li>
                </ul>
                <li>Roblox</li>
                <ul>
                  <li>
                    <a href="https://portworx.com/architects-corner-roblox-runs-platform-70-million-gamers-hashicorp-nomad/">
                      How Roblox runs a platform for 70 million gamers on
                      HashiCorp Nomad
                    </a>
                  </li>
                </ul>
                <li>Oscar Health</li>
                <ul>
                  <li>
                    <a href="https://www.hashicorp.com/resources/scalable-ci-oscar-health-insurance-nomad-docker">
                      Scalable CI at Oscar Health with Nomad and Docker
                    </a>
                  </li>
                </ul>
                <li>eBay</li>
                <ul>
                  <li>
                    <a href="https://www.hashicorp.com/resources/ebay-hashistack-fully-containerized-platform-iac">
                      HashiStack at eBay: A Fully Containerized Platform Based
                      on Infrastructure as Code
                    </a>
                  </li>
                </ul>
                <li>Joyent</li>
                <ul>
                  <li>
                    <a href="https://www.hashicorp.com/resources/autoscaling-hashicorp-nomad">
                      Build Your Own Autoscaling Feature with HashiCorp Nomad
                    </a>
                  </li>
                </ul>
                <li>Dutch National Police</li>
                <ul>
                  <li>
                    <a href="https://www.hashicorp.com/resources/going-cloud-native-at-the-dutch-national-police">
                      Going Cloud-Native at the Dutch National Police
                    </a>
                  </li>
                </ul>
                <li>N26</li>
                <ul>
                  <li>
                    <a href="https://medium.com/insiden26/tech-at-n26-the-bank-in-the-cloud-e5ff818b528b">
                      Tech at N26 - The Bank in the Cloud
                    </a>
                  </li>
                </ul>
                <li>Elsevier</li>
                <ul>
                  <li>
                    <a href="https://www.hashicorp.com/resources/elsevier-nomad-container-framework-demo">
                      Eslevier&apos;s Container Framework with Nomad, Terraform,
                      and Consul
                    </a>
                  </li>
                </ul>
                <li>Graymeta</li>
                <ul>
                  <li>
                    <a href="https://www.hashicorp.com/resources/backend-batch-processing-nomad">
                      Backend Batch Processing At Scale with Nomad
                    </a>
                  </li>
                </ul>
                <li>NIH NCBI</li>
                <ul>
                  <li>
                    <a href="https://www.hashicorp.com/resources/ncbi-legacy-migration-hybrid-cloud-consul-nomad">
                      NCBI’s Legacy Migration to Hybrid Cloud with Consul &amp;
                      Nomad
                    </a>
                  </li>
                </ul>
                <li>Q2EBanking</li>
                <ul>
                  <li>
                    <a href="https://www.youtube.com/watch?v=OsZeKTP2u98&feature=youtu.be&t=1499">
                      Q2&apos;s Nomad Use and Overview
                    </a>
                  </li>
                </ul>
                <li>imgix</li>
                <ul>
                  <li>
                    <a href="https://medium.com/@copyconstruct/schedulers-kubernetes-and-nomad-b0f2e14a896">
                      Cluster Schedulers &amp; Why We Chose Nomad Over
                      Kubernetes
                    </a>
                  </li>
                </ul>
                <li>Region Syddanmark</li>
              </ul>
              <br />
              <p>...and more!</p>
              <h3>Webinars</h3>
              <ul>
                <li>
                  <a href="https://www.hashicorp.com/resources/solutions-engineering-hangout-microservices-with-nomad">
                    Running Microservices with Nomad
                  </a>
                </li>
                <li>
                  <a href="https://www.hashicorp.com/resources/se-hangout-running-heterogeneous-apps-nomad">
                    Running Heterogeneous Apps on Nomad
                  </a>
                </li>
                <li>
                  <a href="https://www.hashicorp.com/resources/supporting-multiple-teams-single-nomad-cluster">
                    Supporting Multiple Teams on a Single Nomad Cluster
                  </a>
                </li>
                <li>
                  <a href="https://www.hashicorp.com/resources/move-your-vmware-workloads-nomad">
                    Moving Your Legacy VMWare Workloads to Nomad
                  </a>
                </li>
                <li>
                  <a href="https://www.hashicorp.com/resources/machine-learning-workflows-hashicorp-nomad-apache-spark">
                    Machine Learning Workflows with HashiCorp Nomad & Apache
                    Spark
                  </a>
                </li>
              </ul>
              <h3>Integrations</h3>
              <ul>
                <li>
                  <a href="https://github.com/jet/damon">
                    <strong>Jet.com</strong> Damon
                  </a>{' '}
                  - A Program to Constrain Windows Executables under Nomad
                  raw_exec
                </li>
                <li>
                  <a href="https://apache.github.io/incubator-heron/docs/operators/deployment/schedulers/nomad/">
                    Apache Heron Nomad Integration
                  </a>{' '}
                  - Realtime, Distributed Stream Processing on Nomad
                </li>
                <li>
                  <a href="https://github.com/jet/nomad-service-alerter">
                    <strong>Jet.com</strong> Nomad Service Alerter
                  </a>{' '}
                  - Alerting for your Nomad services
                </li>
                <li>
                  <a href="https://getnelson.github.io/nelson/">Nelson</a> -
                  Automated, Multi-region Container Deployment with HashiCorp
                  Nomad
                </li>
                <li>
                  <a href="https://github.com/jrasell/chemtrail">Chemtrail</a> -
                  Chemtrail is a client auto scaler allowing for dynamic and
                  safe scaling of the client workerpool based on demand
                </li>
                <li>
                  <a href="https://github.com/jrasell/sherpa">Sherpa</a> - A job
                  scaler for HashiCorp Nomad that aims to be highly flexible so
                  it can support a wide range of architectures and budgets
                </li>
                <li>
                  <a href="https://github.com/jrasell/levant">Levant</a> - An
                  open source templating and deployment tool for HashiCorp Nomad
                  jobs
                </li>
                <li>
                  <a href="https://github.com/underarmour/libra">
                    <strong>Under Armour</strong> Libra
                  </a>{' '}
                  - A Nomad Auto Scaler
                </li>
                <li>
                  <a href="https://docs.datadoghq.com/integrations/nomad">
                    Datadog Nomad Integration
                  </a>{' '}
                  - Monitor Nomad Clusters with Datadog
                </li>
                <li>
                  <a href="https://help.spotinst.com/hc/en-us/articles/115005038289-Nomad-Container-Management">
                    Nomad Container Management with SpotInst
                  </a>{' '}
                  - Nomad Integration for SpotInst Elastigroup
                </li>
                <li>
                  <a href="https://github.com/jenkinsci/nomad-plugin">
                    Jenkins Nomad Plugin
                  </a>{' '}
                  - Nomad Cloud Plugin for Jenkins
                </li>
                <li>
                  OpenFaaS (
                  <a href="https://registry.terraform.io/modules/nicholasjackson/open-faas-nomad/aws/0.4.0">
                    Terraform
                  </a>{' '}
                  | <a href="https://github.com/openfaas/faas">GitHub</a>) -
                  Nomad Integration for OpenFaas (Functions as a Service)
                </li>
                <li>
                  <a href="https://github.com/ValFadeev/rundeck-nomad-plugin">
                    Rundeck Plugin for Nomad
                  </a>{' '}
                  - A Rundeck Plugin for Running Jobs on a Nomad Cluster
                </li>
              </ul>
              <h3>Other</h3>
              <ul>
                <li>
                  <a href="https://github.com/jippi/hashi-ui">hashi-ui</a> -
                  Hashi-UI is a simple to deploy, web based UI for interacting
                  with Nomad and Consul
                </li>
                <li>
                  <a href="https://github.com/jippi/awesome-nomad">
                    jippi/awesome-nomad
                  </a>{' '}
                  - A Curated List of Nomad Tools
                </li>
                <li>
                  <a href="https://github.com/jrasell/nomadfiles">
                    jrasell/nomadfiles
                  </a>{' '}
                  - A Collection of Nomad Job Files for Deploying Applications
                </li>
              </ul>
            </>
          }
        />
      </div>
    </>
  )
}
