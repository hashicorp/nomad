/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { fn } from '@ember/helper';
import { on } from '@ember/modifier';
import { Input } from '@ember/component';
import { pageTitle } from 'ember-page-title';
import and from 'ember-truth-helpers/helpers/and';
import eq from 'ember-truth-helpers/helpers/eq';
import gt from 'ember-truth-helpers/helpers/gt';
import not from 'ember-truth-helpers/helpers/not';
import momentFromNow from 'ember-moment/helpers/moment-from-now';
import LoadingSpinner from 'nomad-ui/components/loading-spinner';
import SingleSelectDropdown from 'nomad-ui/components/single-select-dropdown';
import autofocus from 'nomad-ui/modifiers/autofocus';
import {
  HdsAlert,
  HdsButton,
  HdsFormMaskedInputField,
  HdsPageHeader,
  HdsSeparator,
} from '@hashicorp/design-system-components/components';

<template>
  {{pageTitle (if @controller.tokenRecord "Profile" "Sign In")}}

  <section class="section authorization-page">
    {{#if @controller.isValidatingToken}}
      <LoadingSpinner />
    {{else}}
      <HdsPageHeader as |PH|>
        <PH.Title>
          {{#if @controller.tokenRecord}}
            Profile
          {{else}}
            Sign In
          {{/if}}
        </PH.Title>
        <PH.Actions>
          {{#if @controller.shouldShowPolicies}}
            {{#unless @controller.tokenRecord.isExpired}}
              <HdsButton
                data-test-token-clear
                @size="medium"
                @text="Sign Out"
                @color="critical"
                {{on "click" @controller.clearTokenProperties}}
              />
            {{/unless}}
          {{/if}}
        </PH.Actions>
      </HdsPageHeader>

      <div class="status-notifications">
        {{#if (eq @controller.signInStatus "failure")}}
          <HdsAlert
            data-test-token-error
            @type="inline"
            @color="critical"
            @onDismiss={{@controller.clearSignInStatus}}
            as |A|
          >
            <A.Title>Token Failed to Authenticate</A.Title>
            <A.Description>The token secret you have provided does not match an
              existing token, or has expired.</A.Description>
          </HdsAlert>
        {{/if}}

        {{#if (eq @controller.signInStatus "jwtFailure")}}
          <HdsAlert
            data-test-token-error
            @type="inline"
            @color="critical"
            @onDismiss={{@controller.clearSignInStatus}}
            as |A|
          >
            <A.Title>JWT Failed to Authenticate</A.Title>
            <A.Description>You passed in a JWT, but no JWT auth methods were
              found</A.Description>
          </HdsAlert>
        {{/if}}

        {{#if @controller.tokenRecord.isExpired}}
          <HdsAlert
            data-test-token-expired
            @type="inline"
            @color="critical"
            @onDismiss={{@controller.clearTokenProperties}}
            as |A|
          >
            <A.Title>Your authentication has expired</A.Title>
            <A.Description>Expired
              {{momentFromNow
                @controller.tokenRecord.expirationTime
                interval=1000
              }}
              ({{@controller.tokenRecord.expirationTime}})</A.Description>
          </HdsAlert>
        {{else}}
          {{#if (eq @controller.signInStatus "success")}}
            <HdsAlert
              @onDismiss={{@controller.clearSignInStatus}}
              data-test-token-success
              @type="inline"
              @color="success"
              as |A|
            >
              <A.Title>Token Authenticated!</A.Title>
              <A.Description>Your token is valid and authorized for the
                following policies.</A.Description>
            </HdsAlert>
          {{/if}}
        {{/if}}

        {{#if @controller.token.tokenNotFound}}
          <HdsAlert
            data-test-token-not-found
            @type="inline"
            @color="critical"
            @onDismiss={{@controller.clearTokenNotFound}}
            as |A|
          >
            <A.Title>Token not found</A.Title>
            <A.Description>It may have expired, or been entered incorrectly.</A.Description>
          </HdsAlert>
        {{/if}}

        {{#if @controller.SSOFailure}}
          <HdsAlert
            data-test-sso-error
            @type="inline"
            @color="critical"
            @onDismiss={{@controller.clearState}}
            as |A|
          >
            <A.Title>Failed to sign in with SSO</A.Title>
            <A.Description>Your OIDC provider has failed on sign in; please try
              again or contact your SSO administrator.</A.Description>
          </HdsAlert>
        {{/if}}
      </div>

      {{#if @controller.canSignIn}}
        <div class="sign-in-methods">
          {{#if @controller.nonTokenAuthMethods.length}}
            <h3 class="title is-4">Sign in with SSO</h3>
            <p>Sign in to Nomad using the configured authorization provider.
              After logging in, the policies and rules for the token will be
              listed.</p>
            <div class="sso-auth-methods">
              {{#each @controller.nonTokenAuthMethods as |method|}}
                <HdsButton
                  data-test-auth-method
                  @size="medium"
                  @text="Sign in with {{method.name}}"
                  @color="primary"
                  {{on "click" (fn @controller.redirectToSSO method)}}
                />
              {{/each}}
            </div>
            <span class="or-divider"><span>Or</span></span>
          {{/if}}

          <h3 class="title is-4">Sign in with token</h3>
          <p>Clusters that use Access Control Lists require tokens to perform
            certain tasks. By providing a token Secret ID{{#if
              @controller.hasJWTAuthMethods
            }}
              or
              <a
                href="https://jwt.io/"
                target="_blank"
                rel="noopener noreferrer"
              >JWT</a>{{/if}}, each future request will be authenticated,
            potentially authorizing read access to additional information.</p>
          <label class="label" for="token-input">Secret ID{{#if
              @controller.hasJWTAuthMethods
            }} or JWT{{/if}}</label>
          <div
            class="control
              {{if
                (and
                  @controller.currentSecretIsJWT
                  (gt @controller.JWTAuthMethods.length 1)
                )
                'with-jwt-selector'
              }}"
          >
            <Input
              id="token-input"
              class="input"
              @type="password"
              placeholder="{{if
                @controller.hasJWTAuthMethods
                '36-character token secret or JWT'
                'XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'
              }}"
              {{autofocus}}
              {{on "input" @controller.handleSecretInput}}
              @enter={{@controller.verifyToken}}
              data-test-token-secret
            />

            {{#if @controller.currentSecretIsJWT}}
              {{#if (gt @controller.JWTAuthMethods.length 1)}}
                <SingleSelectDropdown
                  data-test-select-jwt
                  @label="Sign-in method"
                  @options={{@controller.JWTAuthMethodOptions}}
                  @selection={{@controller.selectedJWTAuthMethod}}
                  @onSelect={{@controller.setJWTAuthMethod}}
                />
              {{/if}}
            {{/if}}
          </div>
          <p class="help">Sent with every request to determine authorization</p>
          <HdsButton
            disabled={{not @controller.secret}}
            data-test-token-submit
            @size="medium"
            @text={{if
              @controller.currentSecretIsJWT
              "Sign in with JWT"
              "Sign in with secret"
            }}
            @color="primary"
            {{on "click" @controller.verifyToken}}
          />
        </div>
      {{/if}}

      {{#if @controller.shouldShowPolicies}}
        <div class="token-details">
          {{#unless @controller.tokenRecord.isExpired}}
            <h3 data-test-token-name class="title is-4">Token:
              {{@controller.tokenRecord.name}}</h3>
            <HdsFormMaskedInputField
              readonly
              @isContentMasked={{false}}
              @hasCopyButton={{true}}
              @value={{@controller.tokenRecord.accessor}}
              as |F|
            >
              <F.Label>Accessor ID</F.Label>
            </HdsFormMaskedInputField>
            <HdsFormMaskedInputField
              readonly
              @hasCopyButton={{true}}
              @value={{@controller.tokenRecord.secret}}
              as |F|
            >
              <F.Label>Secret ID</F.Label>
            </HdsFormMaskedInputField>
            {{#if @controller.tokenRecord.expirationTime}}
              <div data-test-token-expiry>Expires:
                {{momentFromNow
                  @controller.tokenRecord.expirationTime
                  interval=1000
                }}
                <span
                  data-test-expiration-timestamp
                >({{@controller.tokenRecord.expirationTime}})</span></div>
            {{/if}}
            {{#if @controller.tokenRecord.roles.length}}
              <HdsSeparator />
              <div>
                <h3 class="title is-4">Roles</h3>
                {{#each @controller.tokenRecord.roles as |role|}}
                  <div data-test-token-role class="boxed-section">
                    <div data-test-role-name class="boxed-section-head">
                      {{role.name}}
                    </div>
                    <div class="boxed-section-body">
                      {{#if role.description}}
                        <p class="content">
                          {{role.description}}
                        </p>
                      {{/if}}
                      <div data-test-role-policies>
                        <h4 class="title is-5">Policies</h4>
                        {{#each role.policies as |policy|}}
                          <li><a
                              href="#{{policy.name}}"
                            >{{policy.name}}</a></li>
                        {{/each}}
                      </div>
                    </div>
                  </div>
                {{/each}}
              </div>
            {{/if}}
            <HdsSeparator />
            <div>
              <h3 class="title is-4">Policies</h3>
              {{#if (eq @controller.tokenRecord.type "management")}}
                <div data-test-token-management-message class="boxed-section">
                  <div class="boxed-section-body has-centered-text">
                    The management token has all permissions
                  </div>
                </div>
              {{else}}
                {{#each @controller.tokenRecord.combinedPolicies as |policy|}}
                  <div
                    id="{{policy.name}}"
                    data-test-token-policy
                    class="boxed-section"
                  >
                    <div data-test-policy-name class="boxed-section-head">
                      {{policy.name}}
                    </div>
                    <div class="boxed-section-body">
                      <p data-test-policy-description class="content">
                        {{#if policy.description}}
                          {{policy.description}}
                        {{else}}
                          <em>No description</em>
                        {{/if}}
                      </p>
                      <pre><code
                          data-test-policy-rules
                        >{{policy.rules}}</code></pre>
                    </div>
                  </div>
                {{/each}}
              {{/if}}
            </div>
          {{/unless}}
        </div>
      {{/if}}
    {{/if}}
  </section>
</template>
