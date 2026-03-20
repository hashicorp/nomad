/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { fn } from '@ember/helper';
import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import { service } from '@ember/service';
import didInsert from '@ember/render-modifiers/modifiers/did-insert';
import can from 'ember-can/helpers/can';
import HamburgerMenu from 'nomad-ui/components/hamburger-menu';
import NomadLogo from 'nomad-ui/components/nomad-logo';
import RegionSwitcher from 'nomad-ui/components/region-switcher';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';

export default class GutterMenu extends Component {
  @service system;
  @service router;
  @service keyboard;

  mainMenuJobsShortcut = ['g', 'j'];
  mainMenuOptimizeShortcut = ['g', 'o'];
  mainMenuStorageShortcut = ['g', 'r'];
  mainMenuVariablesShortcut = ['g', 'v'];
  mainMenuClientsShortcut = ['g', 'c'];
  mainMenuServersShortcut = ['g', 's'];
  mainMenuTopologyShortcut = ['g', 't'];
  mainMenuEvaluationsShortcut = ['g', 'e'];
  mainMenuAdministrationShortcut = ['g', 'a'];

  get isOpen() {
    return this.args.isOpen;
  }

  get onHamburgerClick() {
    return this.args.onHamburgerClick ?? (() => {});
  }

  get sortedNamespaces() {
    const namespaces = this.system.namespaces?.toArray?.() || [];

    return namespaces.sort((a, b) => {
      const aName = a.get('name');
      const bName = b.get('name');

      // Keep default namespace first for parity with prior behavior.
      if (aName === 'default') {
        return -1;
      }
      if (bName === 'default') {
        return 1;
      }

      if (aName < bName) {
        return -1;
      }
      if (aName > bName) {
        return 1;
      }

      return 0;
    });
  }

  transitionTo = (destination) => {
    return this.router.transitionTo(destination);
  };

  <template>
    <div ...attributes>
      <div
        data-test-gutter-menu
        class="page-column is-left {{if this.isOpen 'is-open'}}"
        {{didInsert this.keyboard.registerNav type="main"}}
      >
        <div class="gutter {{if this.isOpen 'is-open'}}">
          <header class="collapsed-menu {{if this.isOpen 'is-open'}}">
            <span
              data-test-gutter-gutter-toggle
              class="gutter-toggle"
              aria-label="menu"
              role="img"
              {{on "click" this.onHamburgerClick}}
            >
              <HamburgerMenu />
            </span>
            <span class="logo-container">
              <NomadLogo />
            </span>
          </header>
          <aside class="menu">
            {{#if this.system.shouldShowRegions}}
              <div class="collapsed-only">
                <p class="menu-label">
                  Region
                  {{if this.system.shouldShowNamespaces "& Namespace"}}
                </p>
                <ul class="menu-list">
                  <li>
                    <div class="menu-item is-wide">
                      <RegionSwitcher />
                    </div>
                  </li>
                </ul>
              </div>
            {{/if}}
            <ul class="menu-list">
              <li
                {{keyboardShortcut
                  menuLevel=true
                  pattern=this.mainMenuJobsShortcut
                }}
              >
                <LinkTo
                  @route="jobs"
                  @activeClass="is-active"
                  data-test-gutter-link="jobs"
                >
                  Jobs
                </LinkTo>
              </li>
              {{#if (can "accept recommendation")}}
                <li
                  {{keyboardShortcut
                    menuLevel=true
                    pattern=this.mainMenuOptimizeShortcut
                    action=(fn this.transitionTo "optimize")
                  }}
                >
                  <LinkTo
                    @route="optimize"
                    @activeClass="is-active"
                    data-test-gutter-link="optimize"
                  >
                    Optimize
                  </LinkTo>
                </li>
              {{/if}}
              <li
                {{keyboardShortcut
                  menuLevel=true
                  pattern=this.mainMenuStorageShortcut
                }}
              >
                <LinkTo
                  @route="storage"
                  @activeClass="is-active"
                  data-test-gutter-link="storage"
                >
                  Storage
                </LinkTo>
              </li>
              {{#if (can "list variables")}}
                <li
                  {{keyboardShortcut
                    menuLevel=true
                    pattern=this.mainMenuVariablesShortcut
                  }}
                >
                  <LinkTo
                    @route="variables"
                    @activeClass="is-active"
                    data-test-gutter-link="variables"
                  >
                    Variables
                  </LinkTo>
                </li>
              {{/if}}
            </ul>
            <p class="menu-label">
              Cluster
            </p>
            <ul class="menu-list">
              <li
                {{keyboardShortcut
                  menuLevel=true
                  pattern=this.mainMenuClientsShortcut
                }}
              >
                <LinkTo
                  @route="clients"
                  @activeClass="is-active"
                  data-test-gutter-link="clients"
                >
                  Clients
                </LinkTo>
              </li>
              <li
                {{keyboardShortcut
                  menuLevel=true
                  pattern=this.mainMenuServersShortcut
                }}
              >
                <LinkTo
                  @route="servers"
                  @activeClass="is-active"
                  data-test-gutter-link="servers"
                >
                  Servers
                </LinkTo>
              </li>
              <li
                {{keyboardShortcut
                  menuLevel=true
                  pattern=this.mainMenuTopologyShortcut
                }}
              >
                <LinkTo
                  @route="topology"
                  @activeClass="is-active"
                  data-test-gutter-link="topology"
                >
                  Topology
                </LinkTo>
              </li>
            </ul>
            <p class="menu-label">
              Operations
            </p>
            <ul class="menu-list">
              <li
                {{keyboardShortcut
                  menuLevel=true
                  pattern=this.mainMenuEvaluationsShortcut
                }}
              >
                <LinkTo
                  @route="evaluations"
                  @activeClass="is-active"
                  data-test-gutter-link="evaluations"
                >
                  Evaluations
                </LinkTo>
              </li>
              {{#if (can "list policies")}}
                <li
                  {{keyboardShortcut
                    menuLevel=true
                    pattern=this.mainMenuAdministrationShortcut
                    action=(fn this.transitionTo "administration")
                  }}
                >
                  <LinkTo
                    @route="administration"
                    @activeClass="is-active"
                    data-test-gutter-link="administration"
                  >
                    Administration
                  </LinkTo>
                </li>
              {{/if}}
            </ul>
          </aside>
          {{#if this.system.agent.version}}
            <footer class="gutter-footer">
              <span class="is-faded">
                v{{this.system.agent.version}}
              </span>
            </footer>
          {{/if}}
        </div>
      </div>
      <div data-test-page-content class="page-column is-right">
        {{yield}}
      </div>
      <div
        data-test-gutter-backdrop
        class="gutter-backdrop {{if this.isOpen 'is-open'}}"
        {{on "click" this.onHamburgerClick}}
      ></div>
    </div>
  </template>
}
