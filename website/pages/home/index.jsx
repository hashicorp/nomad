import Alert from '@hashicorp/react-alert'
import Button from '@hashicorp/react-button'
import VERSION from '../../data/version'

export default function Homepage() {
  return (
    <div id="p-home">
      <section className="hero g-grid-container">
        <Alert
          url="https://www.hashicorp.com/blog/hashicorp-nomad-0-10-general-availability"
          tagColor="nomad-green"
          tag="New"
          text="Announcing the general availability of Nomad 0.10. Read more"
          textColor="dark"
        />

        <img
          className="logo"
          src={require('./img/logo.svg')}
          alt="HashiCorp Nomad Logo"
        />

        <h1 className="g-type-display-3">
          Deploy and Manage Any Containerized, Legacy, or Batch Application
        </h1>
        <div className="buttons">
          <Button url="/intro" title="Get Started" theme={{ brand: 'nomad' }} />
          <Button
            url="/downloads"
            title={`Download ${VERSION}`}
            theme={{ variant: 'secondary' }}
          />
        </div>
      </section>

      <section className="simple-lightweight">
        <div className="g-grid-container">
          <Headline
            text="Simple and Lightweight"
            details="Nomad is an easy-to-use, flexible, and performant workload orchestrator that can deploy a mix of microservice, batch, containerized, and non-containerized applications. Nomad is easy to operate and scale and has native Consul and Vault integrations."
          />

          <TwoColumnList
            items={[
              {
                title: '1. Declare Jobs',
                details:
                  '<p>Users compose and submit high-level job files. Nomad handles the scheduling and upgrading of the applications over time.</p><p>This flexibility makes it easy to deploy one container, dozens of containers, or even <a href="https://www.hashicorp.com/c1m/">millions</a>.</p>'
              },
              {
                title: '2. Plan Changes',
                details:
                  'With built-in dry-run execution, Nomad shows what scheduling decisions it will take before it takes them. Operators can approve or deny these changes to create a safe and reproducible workflow.'
              },
              {
                title: '3. Run Applications',
                details:
                  'Nomad runs applications and ensures they keep running in failure scenarios. In addition to long-running services, Nomad can schedule batch jobs, distributed cron jobs, and parameterized jobs.'
              },
              {
                title: '4. Monitor Progress',
                details:
                  'Stream logs, send signals, and interact with the file system of scheduled applications. These operator-friendly commands bring the familiar debugging tools to a scheduled world.'
              }
            ]}
          />
        </div>
      </section>

      <section className="hybrid-multi-cloud g-grid-container">
        <Headline
          tag="Combine"
          text="Hybrid &amp; Multi-Cloud"
          details="As more organizations adopt cloud technologies, the desire to run in multiple datacenters and multiple regions becomes critical. Nomad can span public and private clouds and treat all your infrastructure as a pool of resources."
        />

        <img
          src={require('./img/hybrid-multi-cloud.svg')}
          alt="List of cloud service provider logos"
        />

        <TwoColumnList
          items={[
            {
              title: 'Hybrid Cloud',
              details:
                'Nomad bridges the gap between the public and private cloud, creating a unified interface for developers to run any application on any infrastructure. Easily run services across your internal private cloud and external public cloud transparently to application developers.'
            },
            {
              title: 'Multi Cloud',
              details:
                'As more organizations adopt cloud technologies, the desire to run applications simultaneously across multiple clouds increases. Nomad combines the power of all cloud providers into a single, unified interface for deploying applications and running services.'
            }
          ]}
        />
      </section>

      <section className="simplify-operations">
        <div className="g-grid-container">
          <Headline
            tag="Maintain"
            text="Simplify Operations"
            details="Nomad simplifies operations by supporting several deployment strategies to safely upgrade jobs, automatically handling machine failures, and providing a single workflow to deploy applications."
            themeColor="green"
          />

          <ul className="left-right-list">
            <li>
              <div className="image">
                <img src={require('./img/maintain-1.svg')} alt="illustration" />
              </div>
              <div className="text">
                <h3 className="g-type-display-4">Rolling Deploys</h3>
                <p className="g-type-body">
                  In order to update an application while reducing downtime,
                  Nomad provides a built-in mechanism for rolling upgrades.
                  Operators specify the rate at which they would like to upgrade
                  their service in the declarative job file, plan the changes,
                  and submit the new job. Nomad then updates the service to the
                  newest version using task health and Consul health check
                  information to ensure replacement allocations are healthy.
                  With full control over the update strategy, rolling upgrades
                  make application deployments simple and safe.
                </p>
              </div>
            </li>
            <li>
              <div className="image">
                <img src={require('./img/maintain-2.svg')} alt="illustration" />
              </div>
              <div className="text">
                <h3 className="g-type-display-4">Blue/Green Deploments</h3>
                <p className="g-type-body">
                  Nomad supports native blue/green and canary deployments
                  through the declarative job file syntax. Instead of doing a
                  rolling upgrade of the existing allocations, the new version
                  of the group is deployed along side the existing set. While
                  this duplicates the resources required during the upgrade
                  process, it allows very safe deployments as the original
                  version of the group is untouched till the operator promotes
                  the new version.
                </p>
              </div>
            </li>
            <li>
              <div className="image">
                <img src={require('./img/maintain-3.svg')} alt="illustration" />
              </div>
              <div className="text">
                <h3 className="g-type-display-4">Automatic Machine Failures</h3>
                <p className="g-type-body">
                  Because it knows the state of all nodes and applications, if a
                  node becomes unhealthy, Nomad will automatically reschedule
                  the applications that were previously running on the unhealthy
                  host onto a new, healthy host if capacity exists. As new
                  clients are added to the cluster, they can share the
                  responsibility of running applications and services.
                </p>
              </div>
            </li>
          </ul>
        </div>
      </section>

      <section className="flexible-workloads g-grid-container">
        <Headline
          tag="Support"
          text="Flexible Workloads"
          details="In addition to supporting Linux, Windows, and Mac, Nomad has
          extensible support for containerized, virtualized, and standalone
          applications. Easily start Docker containers, VMs, or application
          runtimes like Java."
        />

        <img
          src={require('./img/flexible-workloads.svg')}
          alt="List of container and runtime service and operating system logos"
        />

        <TwoColumnList
          items={[
            {
              title: 'Diverse Technologies',
              details:
                'By supporting a wide range of technologies such as Docker, rkt, and LXC, Nomad does not force you into a single technology. Easily use multiple container or virtualization runtimes simultaneously. Applications with their own runtime such as Java or Go can be scheduled directly without the overhead of a container.'
            },
            {
              title: 'Multi OS',
              details:
                'The Nomad client runs on Linux, Windows, and Mac OS. This diverse operating system support enables using the same application scheduler for all your scheduling and runtime needs.'
            }
          ]}
        />
      </section>

      <section className="utilization-costs">
        <div className="g-grid-container">
          <Headline
            tag="Maximize"
            text="Increase Utilization &amp; Reduce Costs"
            details="Nomad uses bin packing to optimize application placement onto servers to maximize resource utilization, increase density, and help reduce costs."
            themeColor="dark"
          />

          <img
            src={require('./img/utilization-costs.svg')}
            alt="illustration - increase density by using nomad"
          />

          <TwoColumnList
            items={[
              {
                title: 'Increase Density',
                details:
                  'Nomad places applications and services based on a bin packing algorithm, which is used to optimize the resource utilization and density of applications. Nomad automatically augments the bin-packing algorithm with anti-affinity rules to avoid colocating instances of the same service to reduce the probability of correlated failures. '
              },
              {
                title: 'Reduce Costs',
                details:
                  'By maximizing resource utilization, Nomad can help remove unused or under-utilized machines in the cluster, reducing overall costs. Additionally, Nomad enables organizations to utilize larger, more cost-effective compute devices which can be less expensive at scale. '
              }
            ]}
          />
        </div>
      </section>

      <section className="examples">
        <div className="g-grid-container">
          <Headline text="Examples" />

          <h3 className="g-type-display-4">Create and Submit Jobs</h3>
          <p className="g-type-body">
            Easily create, validate, submit, and check the status of jobs using
            the Nomad CLI.
          </p>
          <CodeSample>{`<span class='green'>admin@hashicorp.com:</span> nomad init
Example job file written to example.nomad

<span class='green'>admin@hashicorp.com:</span> nomad job validate example.nomad
Job validation successful

<span class='green'>admin@hashicorp.com:</span> nomad job run example.nomad
==> Monitoring evaluation "feb23392
    Evaluation triggered by job "example"
    Evaluation within deployment: "7ffa7ed1"
    Allocation "a2f24fcf" created: node "3714a84a", group "cache"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "feb23392" finished with status "complete"

<span class='green'>admin@hashicorp.com:</span> nomad status
ID       Type     Priority  Status   Submit Date
example  service  50        running  12/20/19 16:29:47 UTC
        `}</CodeSample>

          <h3 className="g-type-display-4">Plan Changes</h3>
          <p className="g-type-body">
            Perform a dry-run to check application placement, scheduling
            decisions, and visualize failures before they happen.
          </p>
          <CodeSample>{`<span class='green'>admin@hashicorp.com:</span> nomad job plan example.nomad
<span class='yellow'>+/-</span> <span class='bold'>Job: "example"</span>
<span class='yellow'>+/-</span> <span class='bold'>Task Group: "cache"</span> (<span class='green'>2 create</span>, <span class='blue'>1 in-place update</span>)
  <span class='yellow'>+/-</span> Count: "1" => "3" (<span class='green'>forces create</span>)
      Task: "redis"

<span class='bold'>Scheduler dry-run:</span>
- <span class='green bold'>All tasks successfully allocated.</span>

<span class='bold'>Job Modify Index: 7</span>

<span class='green'>admin@hashicorp.com</span>: nomad job run -check-index 7 example.nomad
==> Monitoring evaluation "e338a6ae"
    Evaluation triggered by job "example"
    Evaluation within deployment: "43310206"
    Allocation "991a0d89" created: node "c5916a99", group "cache"
    Allocation "daee8be1" created: node "c5916a99", group "cache"
    Allocation "83f29312" modified: node "c5916a99", group "cache"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "e338a6ae" finished with status "complete"
        `}</CodeSample>

          <h3 className="g-type-display-4">View Application Logs</h3>
          <p className="g-type-body">
            Stream application logs directly in the terminal to help analyze and
            debug applications.
          </p>
          <CodeSample>{`<span class='green'>admin@hashicorp.com:</span> nomad alloc logs -tail -job example
20 Dec 16:29:47 # Server started, Redis version 3.2.9
20 Dec 16:29:47* The server is now ready to accept connections on port 6379
        `}</CodeSample>

          <div className="centered">
            <h5 className="g-type-body-large">
              The introduction contains a walkthrough guide, glossary, and a
              range of examples for learning and experimenting with Nomad.{' '}
            </h5>
            <Button
              url="/intro"
              title="Get Started"
              theme={{ brand: 'nomad' }}
            />
          </div>
        </div>
      </section>
    </div>
  )
}

function Headline({ tag, text, details, themeColor }) {
  return (
    <div className="headline">
      {tag && <span className={`tag g-type-label ${themeColor}`}>{tag}</span>}
      <h1 className="g-type-display-2">{text}</h1>
      <h5 className="g-type-body-large lead">{details}</h5>
    </div>
  )
}

function TwoColumnList({ items }) {
  return (
    <ul className="two-column-list">
      {items.map(item => (
        <li key={item.title}>
          <h3 className="g-type-display-4">{item.title}</h3>
          {item.details[0] === '<' ? (
            <span
              className="g-type-body"
              dangerouslySetInnerHTML={{ __html: item.details }}
            />
          ) : (
            <p className="g-type-body">{item.details}</p>
          )}
        </li>
      ))}
    </ul>
  )
}

function CodeSample({ children }) {
  return (
    <pre className="code-sample">
      <div className="chrome">
        <span />
        <span />
        <span />
      </div>
      <code dangerouslySetInnerHTML={{ __html: children }}></code>
    </pre>
  )
}
