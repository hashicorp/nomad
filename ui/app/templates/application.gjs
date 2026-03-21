/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { concat, fn } from '@ember/helper';
import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import BasicDropdownWormhole from 'ember-basic-dropdown/components/basic-dropdown-wormhole';
import FlashMessage from 'ember-cli-flash/components/flash-message';
import { pageTitle } from 'ember-page-title';
import eq from 'ember-truth-helpers/helpers/eq';
import ActionsFlyout from 'nomad-ui/components/actions-flyout';
import ActionsFlyoutGlobalButton from 'nomad-ui/components/actions-flyout-global-button';
import KeyboardShortcutsModal from 'nomad-ui/components/keyboard-shortcuts-modal';
import SvgPatterns from 'nomad-ui/components/svg-patterns';
import { HdsToast } from '@hashicorp/design-system-components/components';

<template>
  <BasicDropdownWormhole />

  {{pageTitle
    (if
      @controller.system.shouldShowRegions
      (concat @controller.system.activeRegion " - ")
    )
    (if
      @controller.system.agent.config.UI.Label.Text
      (concat @controller.system.agent.config.UI.Label.Text " - ")
    )
    "Nomad"
    separator=" - "
  }}
  <SvgPatterns />

  <section class="notifications">
    {{#each @controller.notifications.queue as |flash|}}
      <FlashMessage @flash={{flash}} as |component message close|>
        <HdsToast
          @color={{if message.color message.color "neutral"}}
          @onDismiss={{fn
            @controller.dismissFlash
            close
            message.customCloseAction
          }}
          as |T|
        >
          {{#if message.title}}
            <T.Title>{{message.title}}</T.Title>
          {{/if}}
          {{#if message.message}}
            {{#if message.code}}
              <T.Description><code><pre
                  >{{message.message}}</pre></code></T.Description>
            {{else}}
              <T.Description>{{message.message}}</T.Description>
            {{/if}}
          {{/if}}
          {{#if message.customAction}}
            <T.Button
              @text={{message.customAction.label}}
              @color="secondary"
              {{on "click" message.customAction.action}}
              class={{if message.code "follows-code"}}
            />
          {{/if}}
        </HdsToast>
      </FlashMessage>
    {{/each}}
  </section>

  <KeyboardShortcutsModal />

  <ActionsFlyout />
  <ActionsFlyoutGlobalButton />

  <div id="log-sidebar-portal"></div>

  {{#if @controller.error}}
    <div class="error-container">
      <div data-test-error class="error-message">
        {{#if @controller.isNoLeader}}
          <h1 data-test-error-title class="title is-spaced">No Cluster Leader</h1>
          <p data-test-error-message class="subtitle">
            The cluster has no leader.
            <a
              href="https://developer.hashicorp.com/nomad/docs/manage/outage-recovery"
            >
              Read about Outage Recovery.</a>
          </p>
        {{else if @controller.isOTTExchange}}
          <h1 data-test-error-title class="title is-spaced">Token Exchange Error</h1>
          <p data-test-error-message class="subtitle">
            Failed to exchange the one-time token.
          </p>
        {{else if @controller.is500}}
          <h1 data-test-error-title class="title is-spaced">Server Error</h1>
          <p data-test-error-message class="subtitle">A server error prevented
            data from being sent to the client.</p>
        {{else if @controller.is404}}
          <h1 data-test-error-title class="title is-spaced">Not Found</h1>
          <p data-test-error-message class="subtitle">What you're looking for
            couldn't be found. It either doesn't exist or you are not authorized
            to see it.</p>
        {{else if @controller.is403}}
          <h1 data-test-error-title class="title is-spaced">Not Authorized</h1>
          {{#if @controller.token.secret}}
            <p data-test-error-message class="subtitle">Your
              <LinkTo
                @route="settings.tokens"
                data-test-error-acl-link
                {{on
                  "click"
                  (fn
                    @controller.setPostExpiryPath @controller.router.currentURL
                  )
                }}
              >ACL token</LinkTo>
              does not provide the required permissions. Contact your
              administrator if this is an error.</p>
          {{else}}
            <p data-test-error-message class="subtitle">Provide an
              <LinkTo
                @route="settings.tokens"
                data-test-error-acl-link
                {{on
                  "click"
                  (fn
                    @controller.setPostExpiryPath @controller.router.currentURL
                  )
                }}
              >ACL token</LinkTo>
              with requisite permissions to view this.</p>
          {{/if}}
        {{else}}
          <h1 data-test-error-title class="title is-spaced">Error</h1>
          <p data-test-error-message class="subtitle">Something went wrong.</p>
        {{/if}}
        {{#if (eq @controller.config.environment "development")}}
          <pre class="error-stack-trace"><code
            >{{@controller.errorStr}}</code></pre>
        {{/if}}
      </div>
      <div class="error-links">
        <LinkTo
          @route="jobs"
          data-test-error-jobs-link
          class="button is-white"
        >Go to Jobs</LinkTo>
        <LinkTo
          @route="clients"
          data-test-error-clients-link
          class="button is-white"
        >Go to Clients</LinkTo>
        <LinkTo
          @route="settings.tokens"
          data-test-error-signin-link
          class="button is-white"
          {{on
            "click"
            (fn @controller.setPostExpiryPath @controller.router.currentURL)
          }}
        >Go to Sign In</LinkTo>
      </div>
    </div>
  {{else}}
    {{outlet}}
  {{/if}}
</template>
