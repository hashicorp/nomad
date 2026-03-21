/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { fn } from '@ember/helper';
import { on } from '@ember/modifier';
import { pageTitle } from 'ember-page-title';

<template>
  {{pageTitle "Mock OIDC Test Page"}}

  <section class="mock-sso-provider">
    <h1>OIDC Test route: {{@controller.auth_method}}</h1>
    <h2>(Mirage only)</h2>
    <div class="providers">
      {{#each @model as |fakeAccount|}}
        <button
          type="button"
          class="button"
          data-test-oidc-account={{fakeAccount.name}}
          {{on "click" (fn @controller.signIn fakeAccount)}}
        >
          Sign In as
          {{fakeAccount.name}}
        </button>
      {{/each}}
      <button
        type="button"
        class="button error"
        {{on "click" @controller.failToSignIn}}
      >
        Simulate Failure
      </button>
    </div>
  </section>
  {{outlet}}
</template>
