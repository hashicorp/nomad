/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import { htmlSafe } from '@ember/template';
import { service } from '@ember/service';
import media from 'ember-responsive/helpers/media';
import GlobalSearchControl from 'nomad-ui/components/global-search/control';
import HamburgerMenu from 'nomad-ui/components/hamburger-menu';
import NomadLogo from 'nomad-ui/components/nomad-logo';
import ProfileNavbarItem from 'nomad-ui/components/profile-navbar-item';
import RegionSwitcher from 'nomad-ui/components/region-switcher';

export default class GlobalHeader extends Component {
  @service system;

  get onHamburgerClick() {
    return this.args.onHamburgerClick ?? (() => {});
  }

  get labelStyles() {
    return htmlSafe(
      `
        color: ${this.system.agent.get('config')?.UI?.Label?.TextColor};
        background-color: ${
          this.system.agent.get('config')?.UI?.Label?.BackgroundColor
        };
      `,
    );
  }

  <template>
    <div data-test-global-header ...attributes>
      {{! template-lint-disable no-duplicate-landmark-elements }}
      <nav class="navbar is-primary" title="navigation">
        <div class="navbar-brand">
          <span
            data-test-header-gutter-toggle
            class="gutter-toggle"
            aria-label="menu"
            role="img"
            {{on "click" this.onHamburgerClick}}
          >
            <HamburgerMenu />
          </span>
          <LinkTo @route="jobs" class="navbar-item is-logo" aria-label="Home">
            <NomadLogo />
          </LinkTo>
          {{#if this.system.agent.config.UI.Label.Text}}
            <div class="custom-label" style={{this.labelStyles}}>
              {{this.system.agent.config.UI.Label.Text}}
            </div>
          {{/if}}
        </div>
        {{#if this.system.fuzzySearchEnabled}}
          {{! template-lint-disable simple-unless }}
          {{#unless (media "isMobile")}}
            <GlobalSearchControl />
          {{/unless}}
        {{/if}}
        <div class="navbar-end">
          {{#if this.system.agent.config.UI.Consul.BaseUIURL}}
            <a
              data-test-header-consul-link
              href={{this.system.agent.config.UI.Consul.BaseUIURL}}
              target="_blank"
              rel="noopener noreferrer"
              class="navbar-item"
            >
              Consul
            </a>
          {{/if}}
          {{#if this.system.agent.config.UI.Vault.BaseUIURL}}
            <a
              data-test-header-vault-link
              href={{this.system.agent.config.UI.Vault.BaseUIURL}}
              target="_blank"
              rel="noopener noreferrer"
              class="navbar-item"
            >
              Vault
            </a>
          {{/if}}
          <a
            href="https://developer.hashicorp.com/nomad/docs"
            target="_blank"
            rel="noopener noreferrer"
            class="navbar-item"
          >
            Documentation
          </a>
          <ProfileNavbarItem />
        </div>
      </nav>
      <div class="navbar is-secondary">
        <div class="navbar-item is-gutter">
          <RegionSwitcher @decoration="is-outlined" />
        </div>
        <nav class="breadcrumb is-large" title="breadcrumb navigation">
          <ul>
            {{yield}}
          </ul>
        </nav>
      </div>
    </div>
  </template>
}
