import React, { useState } from 'react'

export default function TemporaryMegaNavSuite({ product }) {
  const [open, setOpen] = useState(false)

  function activeProduct(name) {
    return name.toLowerCase() === (product || '').toLowerCase()
      ? 'is-active'
      : ''
  }

  return (
    <div className="hidden-print mega-nav-sandbox">
      <svg
        className="hidden-print"
        aria-hidden="true"
        style={{ position: 'absolute', width: 0, height: 0 }}
        width="0"
        height="0"
        xmlns="http://www.w3.org/2000/svg"
      >
        <defs>
          <symbol id="mega-nav-icon-angle" viewBox="0 0 13 7">
            <path d="M6.5 6.5L.7 1.6l.6-.8 5.2 4.4L11.7.8l.6.8z" />
          </symbol>
          <symbol id="mega-nav-icon-close" viewBox="0 0 13 15">
            <path
              vectorEffect="non-scaling-stroke"
              d="M1.1 14L11.9 1m0 13L1.1 1"
            />
          </symbol>
        </defs>
      </svg>
      <div className="mega-nav-banner">
        <div className="container">
          <div className="mega-nav-banner-item">
            <p className="mega-nav-tagline">
              <span className="hidden-xs text-muted">
                {product
                  ? `Learn how ${product} fits into the`
                  : 'Learn more about the'}
              </span>
            </p>
            <div id="#mega-nav" className={`mega-nav ${open ? 'open' : ''}`}>
              <button
                type="button"
                id="mega-nav-ctrl"
                className="mega-nav-ctrl"
                onClick={() => {
                  console.log('clicked')
                  setOpen(!open)
                }}
              >
                <div className="mega-nav-ctrl-items">
                  <img
                    src={require('./img/temporary_suite/logo-hashicorp.svg')}
                    alt="HashiCorp Logo"
                  />
                  <strong>HashiCorp Stack</strong>
                  <span className="mega-nav-icon-outline">
                    <svg
                      className="mega-nav-icon mega-nav-icon-angle-right"
                      aria-hidden="true"
                    >
                      <use
                        xmlnsXlink="http://www.w3.org/1999/xlink"
                        xlinkHref="#mega-nav-icon-angle"
                      />
                    </svg>
                    <span className="visuallyhidden">Open</span>
                  </span>
                </div>
              </button>
              <div
                id="mega-nav-body-ct"
                className="mega-nav-body-ct"
                aria-labelledby="mega-nav-ctrl"
              >
                <div className="mega-nav-body">
                  <button
                    type="button"
                    id="mega-nav-close"
                    className="mega-nav-close"
                  >
                    <svg
                      className="mega-nav-icon mega-nav-icon-close"
                      aria-hidden="true"
                    >
                      <use
                        xmlnsXlink="http://www.w3.org/1999/xlink"
                        xlinkHref="#mega-nav-icon-close"
                      />
                    </svg>
                    <span className="visuallyhidden">Close</span>
                  </button>
                  <div className="mega-nav-body-header">
                    <div className="mega-nav-body-header-item">
                      <h2 className="mega-nav-h1">
                        Provision, Secure, Connect, and&nbsp;Run
                      </h2>
                      <p className="mega-nav-h2">
                        Any infrastructure for any&nbsp;application
                      </p>
                    </div>
                    <div className="mega-nav-body-header-item">
                      <a
                        href="https://www.hashicorp.com/"
                        className="mega-nav-btn"
                      >
                        <img
                          src={require('./img/temporary_suite/logo-hashicorp.svg')}
                          alt="HashiCorp Logo"
                        />{' '}
                        Learn the HashiCorp Enterprise&nbsp;Stack{' '}
                        <svg
                          className="mega-nav-icon mega-nav-icon-angle-right"
                          aria-hidden="true"
                        >
                          <use
                            xmlnsXlink="http://www.w3.org/1999/xlink"
                            xlinkHref="#mega-nav-icon-angle"
                          />
                        </svg>
                      </a>
                    </div>
                  </div>
                  <div className="mega-nav-body-grid">
                    <div className="mega-nav-body-grid-item">
                      <h3 className="mega-nav-h3">Provision</h3>
                      <ul className="mega-nav-grid">
                        <li>
                          <a
                            href="https://www.vagrantup.com"
                            className={`mega-nav-grid-item mega-nav-grid-item-vagrant ${activeProduct(
                              'vagrant'
                            )}`}
                          >
                            <div className="mega-nav-grid-item-img">
                              <img
                                src={require('./img/temporary_suite/logo-vagrant.svg')}
                                alt="Vagrant Logo"
                              />
                            </div>
                            <b>Vagrant</b>
                            <ul>
                              <li>
                                <span className="mega-nav-tag">Build</span>
                              </li>
                              <li>
                                <span className="mega-nav-tag">Test</span>
                              </li>
                            </ul>
                          </a>
                        </li>
                        <li>
                          <a
                            href="https://www.packer.io"
                            className={`mega-nav-grid-item mega-nav-grid-item-packer ${activeProduct(
                              'packer'
                            )}`}
                          >
                            <div className="mega-nav-grid-item-img">
                              <img
                                src={require('./img/temporary_suite/logo-packer.svg')}
                                alt="Packer Logo"
                              />
                            </div>
                            <b>Packer</b>
                            <ul>
                              <li>
                                <span className="mega-nav-tag">Package</span>
                              </li>
                            </ul>
                          </a>
                        </li>
                        <li>
                          <a
                            href="https://www.terraform.io"
                            className={`mega-nav-grid-item mega-nav-grid-item-terraform ${activeProduct(
                              'terraform'
                            )}`}
                          >
                            <div className="mega-nav-grid-item-img">
                              <img
                                src={require('./img/temporary_suite/logo-terraform.svg')}
                                alt="Terraform Logo"
                              />
                            </div>
                            <b>Terraform</b>
                            <ul>
                              <li>
                                <span className="mega-nav-tag">Provision</span>
                              </li>
                            </ul>
                          </a>
                        </li>
                      </ul>
                    </div>
                    <div className="mega-nav-body-grid-item">
                      <h3 className="mega-nav-h3">Secure</h3>
                      <ul className="mega-nav-grid">
                        <li>
                          <a
                            href="https://www.vaultproject.io"
                            className={`mega-nav-grid-item mega-nav-grid-item-vault ${activeProduct(
                              'vault'
                            )}`}
                          >
                            <div className="mega-nav-grid-item-img">
                              <img
                                src={require('./img/temporary_suite/logo-vault.svg')}
                                alt="Vault Logo"
                              />
                            </div>
                            <b>Vault</b>
                            <ul>
                              <li>
                                <span className="mega-nav-tag">Secure</span>
                              </li>
                            </ul>
                          </a>
                        </li>
                      </ul>
                    </div>
                    <div className="mega-nav-body-grid-item">
                      <h3 className="mega-nav-h3">Connect</h3>
                      <ul className="mega-nav-grid">
                        <li>
                          <a
                            href="https://www.consul.io"
                            className="mega-nav-grid-item mega-nav-grid-item-consul"
                          >
                            <div className="mega-nav-grid-item-img">
                              <img
                                src={require('./img/temporary_suite/logo-consul.svg')}
                                alt="Consul Logo"
                              />
                            </div>
                            <b>Consul</b>
                            <ul>
                              <li>
                                <span className="mega-nav-tag">Connect</span>
                              </li>
                            </ul>
                          </a>
                        </li>
                      </ul>
                    </div>
                    <div className="mega-nav-body-grid-item">
                      <h3 className="mega-nav-h3">Run</h3>
                      <ul className="mega-nav-grid">
                        <li>
                          <a
                            href="https://www.nomadproject.io"
                            className={`mega-nav-grid-item mega-nav-grid-item-nomad ${activeProduct(
                              'nomad'
                            )}`}
                          >
                            <div className="mega-nav-grid-item-img">
                              <img
                                src={require('./img/temporary_suite/logo-nomad.svg')}
                                alt="Nomad Logo"
                              />
                            </div>
                            <b>Nomad</b>
                            <ul>
                              <li>
                                <span className="mega-nav-tag">Run</span>
                              </li>
                            </ul>
                          </a>
                        </li>
                      </ul>
                    </div>
                  </div>
                  <div className="mega-nav-body-footer">
                    <p>Seven elements of the modern Application Lifecycle</p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
