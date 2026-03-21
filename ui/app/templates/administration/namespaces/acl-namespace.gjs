/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, hash } from '@ember/helper';
import { pageTitle } from 'ember-page-title';
import can from 'ember-can/helpers/can';
import perform from 'ember-concurrency/helpers/perform';
import and from 'ember-truth-helpers/helpers/and';
import notEq from 'ember-truth-helpers/helpers/not-eq';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import NamespaceEditor from 'nomad-ui/components/namespace-editor';
import TwoStepButton from 'nomad-ui/components/two-step-button';
import {
  HdsAlert,
  HdsLinkInline,
  HdsPageHeader,
} from '@hashicorp/design-system-components/components';

<template>
  <Breadcrumb
    @crumb={{hash
      label=@model.name
      args=(array "administration.namespaces.acl-namespace" @model.name)
    }}
  />
  {{pageTitle "Namespace"}}

  <section class="section">
    <HdsPageHeader as |PH|>
      <PH.Title data-test-title>{{@model.name}}</PH.Title>
      {{#if (and (notEq @model.name "default") (can "destroy namespace"))}}
        <PH.Actions>
          <TwoStepButton
            data-test-delete-namespace
            @idleText="Delete Namespace"
            @cancelText="Cancel"
            @confirmText="Yes, Delete Namespace"
            @confirmationMessage="Are you sure?"
            @awaitingConfirmation={{@controller.deleteNamespace.isRunning}}
            @disabled={{@controller.deleteNamespace.isRunning}}
            @onConfirm={{perform @controller.deleteNamespace}}
          />
        </PH.Actions>
      {{/if}}
    </HdsPageHeader>

    <HdsAlert
      @type="inline"
      @color="highlight"
      @icon="info"
      class="related-entities notification"
      as |A|
    >
      <A.Title>Related Resources</A.Title>
      <A.Description>
        View this namespace's
        <HdsLinkInline
          @route="jobs"
          @query={{hash namespace=@model.name}}
        >jobs</HdsLinkInline>
        or
        <HdsLinkInline
          @route="variables"
          @query={{hash namespace=@model.name}}
        >variables</HdsLinkInline>.
      </A.Description>
    </HdsAlert>

    <NamespaceEditor @namespace={{@model}} />
  </section>
</template>
