/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, hash } from '@ember/helper';
import { LinkTo } from '@ember/routing';
import { pageTitle } from 'ember-page-title';
import { HdsPageHeader } from '@hashicorp/design-system-components/components';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import RoleEditor from 'nomad-ui/components/role-editor';

<template>
  <Breadcrumb
    @crumb={{hash label="New" args=(array "administration.roles.new")}}
  />
  {{pageTitle "Create Role"}}
  <section class="section">
    <HdsPageHeader as |PH|>
      <PH.Title data-test-title>Create Role</PH.Title>
    </HdsPageHeader>
    {{#if @model.policies.length}}
      <RoleEditor @role={{@model.role}} @policies={{@model.policies}} />
    {{else}}
      <div class="empty-message">
        <h3 class="empty-message-headline">
          No Policies
        </h3>
        <p class="empty-message-body">
          At least one Policy is required to create a Role;
          <LinkTo @route="administration.policies.new">create a new policy</LinkTo>
        </p>
      </div>
    {{/if}}
  </section>
</template>
