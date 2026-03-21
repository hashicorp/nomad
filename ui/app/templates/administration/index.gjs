/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { LinkTo } from '@ember/routing';
import can from 'ember-can/helpers/can';
import {
  HdsButton,
  HdsCardContainer,
  HdsLinkStandalone,
} from '@hashicorp/design-system-components/components';
import pluralize from 'nomad-ui/helpers/pluralize';

<template>
  <section class="section access-control-overview">
    <section class="intro">
      <p>Your Nomad cluster has Access Control enabled, which you can use to
        control access to data and APIs. Here, you can manage the Tokens,
        Policies, and Roles for your system.</p>
      <footer>
        <HdsLinkStandalone
          @icon="docs-link"
          @text="ACL System Fundamentals"
          @iconPosition="trailing"
          @href="https://developer.hashicorp.com/nomad/docs/secure/acl"
        />
        <HdsLinkStandalone
          @icon="docs-link"
          @text="ACL Policy Concepts"
          @iconPosition="trailing"
          @href="https://developer.hashicorp.com/nomad/docs/secure/acl/policies"
        />
      </footer>
    </section>
    <div class="section-cards">
      <HdsCardContainer @level="mid" @hasBorder={{true}} data-test-tokens-card>
        <LinkTo @route="administration.tokens">
          {{@model.tokens.length}}
          {{pluralize "Token" @model.tokens.length}}
        </LinkTo>
        <p>User access tokens are associated with one or more policies or roles
          to grant specific capabilities.</p>
        <HdsButton
          @text="Create Token"
          @color="secondary"
          @iconPosition="leading"
          @icon="plus"
          @route="administration.tokens.new"
        />
      </HdsCardContainer>
      <HdsCardContainer @level="mid" @hasBorder={{true}} data-test-roles-card>
        <LinkTo @route="administration.roles">
          {{@model.roles.length}}
          {{pluralize "Role" @model.roles.length}}
        </LinkTo>
        <p>Roles group one or more Policies into higher-level sets of
          permissions.</p>
        <HdsButton
          @text="Create Role"
          @color="secondary"
          @iconPosition="leading"
          @icon="plus"
          @route="administration.roles.new"
        />
      </HdsCardContainer>
      <HdsCardContainer
        @level="mid"
        @hasBorder={{true}}
        data-test-policies-card
      >
        <LinkTo @route="administration.policies">
          {{@model.policies.length}}
          {{pluralize "Policy" @model.policies.length}}
        </LinkTo>
        <p>Sets of rules defining the capabilities granted to adhering tokens.</p>
        <HdsButton
          @text="Create Policy"
          @color="secondary"
          @iconPosition="leading"
          @icon="plus"
          @route="administration.policies.new"
        />
      </HdsCardContainer>
      <HdsCardContainer
        @level="mid"
        @hasBorder={{true}}
        data-test-namespaces-card
      >
        <LinkTo @route="administration.namespaces">
          {{@model.namespaces.length}}
          {{pluralize "Namespace" @model.namespaces.length}}
        </LinkTo>
        <p>Namespaces allow jobs and other objects to be segmented from each
          other.</p>
        <HdsButton
          @text="Create Namespace"
          @color="secondary"
          @iconPosition="leading"
          @icon="plus"
          @route="administration.namespaces.new"
        />
      </HdsCardContainer>
      {{#if (can "read sentinel-policy")}}
        <HdsCardContainer
          @level="mid"
          @hasBorder={{true}}
          data-test-sentinel-policies-card
        >
          <LinkTo @route="administration.sentinel-policies">
            {{@model.sentinelPolicies.length}}
            {{pluralize "Sentinel Policy" @model.sentinelPolicies.length}}
          </LinkTo>
          <p>Sentinel Policies allow operators to express rules as code and have
            those rules automatically enforced when jobs are planned.</p>
          <HdsButton
            @text="Create Sentinel Policy"
            @color="secondary"
            @iconPosition="leading"
            @icon="plus"
            @route="administration.sentinel-policies.new"
          />
        </HdsCardContainer>
      {{/if}}
    </div>
  </section>
  {{outlet}}
</template>
