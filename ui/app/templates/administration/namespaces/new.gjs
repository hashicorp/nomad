/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, hash } from '@ember/helper';
import { pageTitle } from 'ember-page-title';
import { HdsPageHeader } from '@hashicorp/design-system-components/components';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import NamespaceEditor from 'nomad-ui/components/namespace-editor';

<template>
  <Breadcrumb
    @crumb={{hash label="New" args=(array "administration.namespaces.new")}}
  />
  {{pageTitle "Create Namespace"}}
  <section class="section">
    <HdsPageHeader as |PH|>
      <PH.Title>Create Namespace</PH.Title>
    </HdsPageHeader>
    <NamespaceEditor @namespace={{@model}} />
  </section>
</template>
