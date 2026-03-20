/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { array, fn, hash } from '@ember/helper';
import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import { service } from '@ember/service';
import { task } from 'ember-concurrency';
import can from 'ember-can/helpers/can';
import { findBy } from '@nullvoxpopuli/ember-composable-helpers';
import { eq, not } from 'ember-truth-helpers';
import {
  HdsButton,
  HdsFormMaskedInputField,
  HdsFormRadioGroup,
  HdsLinkInline,
  HdsTable,
  HdsTag,
} from '@hashicorp/design-system-components/components';
import momentFromNow from 'ember-moment/helpers/moment-from-now';
import autofocus from 'nomad-ui/modifiers/autofocus';
import Tooltip from 'nomad-ui/components/tooltip';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';

export default class TokenEditor extends Component {
  @service notifications;
  @service router;
  @service store;
  @service system;

  @tracked tokenPolicies = [];
  @tracked tokenRoles = [];
  @tracked tokenRegion = '';

  constructor() {
    super(...arguments);
    this.tokenPolicies = this.args.token.policies.toArray() || [];
    this.tokenRoles = this.args.token.roles.toArray() || [];
    if (this.args.token.isNew) {
      this.args.token.expirationTTL = 'never';
    }
    this.tokenRegion = this.system.activeRegion;
  }

  policyKey(policy) {
    return policy?.name;
  }

  roleKey(role) {
    return role?.id || role?.name;
  }

  updateTokenName = ({ target: { value } }) => {
    this.args.token.set('name', value);
  };

  updateTokenPolicies = (policy, event) => {
    const { checked } = event.target;
    const key = this.policyKey(policy);

    if (checked) {
      if (!this.tokenPolicies.some((item) => this.policyKey(item) === key)) {
        this.tokenPolicies = [...this.tokenPolicies, policy];
      }
    } else {
      this.tokenPolicies = this.tokenPolicies.filter(
        (item) => this.policyKey(item) !== key,
      );
    }
  };

  updateTokenRoles = (role, event) => {
    const { checked } = event.target;
    const key = this.roleKey(role);

    if (checked) {
      if (!this.tokenRoles.some((item) => this.roleKey(item) === key)) {
        this.tokenRoles = [...this.tokenRoles, role];
      }
    } else {
      this.tokenRoles = this.tokenRoles.filter(
        (item) => this.roleKey(item) !== key,
      );
    }
  };

  updateTokenType = (event) => {
    this.args.token.type = event.target.id;
  };

  updateTokenExpirationTime = (event) => {
    const rawValue = event?.target?.value;
    if (!rawValue) {
      return;
    }

    const normalizedValue = rawValue.includes('.')
      ? rawValue.split('.')[0]
      : rawValue;
    const parsed = new Date(normalizedValue);

    if (Number.isNaN(parsed.getTime())) {
      return;
    }

    this.args.token.expirationTTL = null;
    this.args.token.expirationTime = parsed;
  };

  updateTokenExpirationTTL = (event) => {
    this.args.token.expirationTime = null;
    if (event.target.value === 'never') {
      this.args.token.expirationTTL = null;
    } else if (event.target.value === 'custom') {
      this.args.token.expirationTime = new Date();
    } else {
      this.args.token.expirationTTL = event.target.value;
    }
  };

  updateTokenLocality = (event) => {
    this.tokenRegion = event.target.id;
  };

  save = task({ drop: true }, async (event) => {
    event?.preventDefault?.();

    const activeToken = this.args.token;

    try {
      const shouldRedirectAfterSave = activeToken.isNew;

      const policyIDs = this.tokenPolicies
        .map((policy) => policy?.id || policy?.name)
        .filter(Boolean);
      const roleIDs = this.tokenRoles
        .map((role) => role?.id || role?.name)
        .filter(Boolean);

      activeToken.policies = this.tokenPolicies;
      activeToken.roles = this.tokenRoles;
      activeToken.policyIDs = policyIDs;
      activeToken.policyNames = policyIDs;
      activeToken.roleIDs = roleIDs;

      if (activeToken.type === 'management') {
        activeToken.policyIDs = [];
        activeToken.policyNames = [];
        activeToken.roleIDs = [];
        activeToken.policies = [];
        activeToken.roles = [];
      }

      activeToken.global = this.tokenRegion === 'global';

      if (activeToken.expirationTTL === 'never') {
        activeToken.expirationTTL = null;
      }

      const adapterRegion = activeToken.global
        ? this.system.get('defaultRegion.region')
        : this.tokenRegion;

      await activeToken.save({
        adapterOptions: adapterRegion ? { region: adapterRegion } : {},
      });

      this.notifications.add({
        title: 'Token Saved',
        color: 'success',
      });

      if (shouldRedirectAfterSave) {
        this.router.transitionTo('administration.tokens.token', activeToken.id);
      }
    } catch (err) {
      const message = err.errors?.length
        ? messageFromAdapterError(err)
        : err.message;

      this.notifications.add({
        title: `Error creating Token ${activeToken.name}`,
        message,
        color: 'critical',
        sticky: true,
      });
    }
  });

  <template>
    <form
      class="acl-form"
      autocomplete="off"
      {{on "submit" this.save.perform}}
      ...attributes
    >
      <label>
        <span class="hds-form-label">Token Name</span>
        <input
          data-test-token-name-input
          type="text"
          value={{@token.name}}
          class="input"
          {{autofocus ignore=(not @token.isNew)}}
          {{on "input" this.updateTokenName}}
        />
      </label>

      <div class="expiration-time">
        {{#if @token.isNew}}
          <span class="hds-form-label">Expiration time</span>

          <HdsFormRadioGroup
            @layout="horizontal"
            @name="expiration-time"
            {{on "change" this.updateTokenExpirationTTL}}
            as |G|
          >
            <G.RadioField @id="10m" @value="10m" as |F|>
              <F.Label>10 minutes</F.Label>
            </G.RadioField>
            <G.RadioField @id="8h" @value="8h" as |F|>
              <F.Label>8 hours</F.Label>
            </G.RadioField>
            <G.RadioField @id="24h" @value="24h" as |F|>
              <F.Label>24 hours</F.Label>
            </G.RadioField>
            <G.RadioField
              @id="never"
              @value="never"
              checked={{eq @token.expirationTTL "never"}}
              as |F|
            >
              <F.Label>Never</F.Label>
            </G.RadioField>
            <G.RadioField @id="custom" @value="custom" as |F|>
              <F.Label>Custom</F.Label>
            </G.RadioField>
          </HdsFormRadioGroup>

          {{#if @token.expirationTime}}
            <input
              data-test-token-expiration-time-input
              type="datetime-local"
              id="token-expiration-time"
              step="any"
              class="input token-expiration-datetime-input"
              {{on "change" this.updateTokenExpirationTime}}
            />
          {{/if}}
        {{else}}
          <span class="hds-form-label">
            {{#if @token.expirationTime}}
              Token
              {{#if @token.isExpired}}expired{{else}}expires{{/if}}
              <Tooltip @text={{@token.expirationTime}} @isFullText={{true}}>
                <span
                  data-test-token-expiration-time
                  class={{if @token.isExpired "has-text-danger"}}
                >{{momentFromNow @token.expirationTime interval=1000}}</span>
              </Tooltip>
            {{else}}
              Token never expires
            {{/if}}
          </span>
        {{/if}}
      </div>

      {{#unless @token.isNew}}
        <div>
          <HdsFormMaskedInputField
            @isContentMasked={{false}}
            @hasCopyButton={{true}}
            @value={{@token.accessor}}
            readonly
            data-test-token-accessor
            as |F|
          >
            <F.Label>Token Accessor</F.Label>
          </HdsFormMaskedInputField>
        </div>

        <div>
          <HdsFormMaskedInputField
            @hasCopyButton={{true}}
            @value={{@token.secret}}
            readonly
            data-test-token-secret
            as |F|
          >
            <F.Label>Token Secret</F.Label>
          </HdsFormMaskedInputField>
        </div>
      {{/unless}}

      {{#if @token.isNew}}
        {{#if this.system.shouldShowRegions}}
          <HdsFormRadioGroup
            data-test-global-token-group
            @layout="horizontal"
            @name="regional-or-global"
            {{on "change" this.updateTokenLocality}}
            as |G|
          >
            <G.Legend>Token Region</G.Legend>
            <G.HelperText>See
              <HdsLinkInline
                @href="https://developer.hashicorp.com/nomad/docs/secure/acl/tokens#token-replication-settings"
              >ACL token fundamentals: Token replication settings</HdsLinkInline>
              for more information.</G.HelperText>
            <G.RadioField
              @id={{this.system.activeRegion}}
              checked={{eq this.tokenRegion this.system.activeRegion}}
              data-test-locality="active-region"
              as |F|
            >
              <F.Label
                data-test-active-region-label
              >{{this.system.activeRegion}}</F.Label>
            </G.RadioField>
            {{#if this.system.defaultRegion.region}}
              {{#unless
                (eq this.system.activeRegion this.system.defaultRegion.region)
              }}
                <G.RadioField
                  @id={{this.system.defaultRegion.region}}
                  checked={{eq
                    this.tokenRegion
                    this.system.defaultRegion.region
                  }}
                  data-test-locality="default-region"
                  as |F|
                >
                  <F.Label>{{this.system.defaultRegion.region}}
                    (authoritative region)</F.Label>
                </G.RadioField>
              {{/unless}}
            {{/if}}
            <G.RadioField
              @id="global"
              checked={{eq this.tokenRegion "global"}}
              data-test-locality="global"
              as |F|
            >
              <F.Label>global</F.Label>
            </G.RadioField>
          </HdsFormRadioGroup>
        {{/if}}
      {{/if}}

      <div>
        <HdsFormRadioGroup
          @layout="horizontal"
          @name="token-type"
          {{on "change" this.updateTokenType}}
          as |G|
        >
          <G.Legend>Client or Management token?</G.Legend>
          <G.HelperText>See
            <HdsLinkInline
              @href="https://developer.hashicorp.com/nomad/docs/secure/acl/tokens#token-types"
            >Token types documentation</HdsLinkInline>
            for more information.</G.HelperText>
          <G.RadioField
            @id="client"
            checked={{eq @token.type "client"}}
            data-test-token-type="client"
            as |F|
          >
            <F.Label>Client</F.Label>
          </G.RadioField>
          <G.RadioField
            @id="management"
            checked={{eq @token.type "management"}}
            data-test-token-type="management"
            as |F|
          >
            <F.Label>Management</F.Label>
          </G.RadioField>
        </HdsFormRadioGroup>
      </div>

      {{#if (eq @token.type "client")}}
        <div data-test-token-policies>
          <label>
            Policies
          </label>
          {{#if @policies.length}}
            <HdsTable
              @caption="A list of policies available to this token"
              class="acl-table"
              @model={{@policies}}
              @columns={{array
                (hash key="selected" width="80px")
                (hash key="name" label="Name" isSortable=true)
                (hash key="description" label="Description")
                (hash key="definition" label="View Policy Definition")
              }}
              @sortBy="name"
            >
              <:body as |B|>
                <B.Tr>
                  <B.Td class="selection-checkbox">
                    <label>
                      <input
                        type="checkbox"
                        checked={{findBy "name" B.data.name @token.policies}}
                        {{on "change" (fn this.updateTokenPolicies B.data)}}
                      />
                    </label>
                  </B.Td>
                  <B.Td data-test-policy-name>{{B.data.name}}</B.Td>
                  <B.Td>{{B.data.description}}</B.Td>
                  <B.Td>
                    <LinkTo
                      @route="administration.policies.policy"
                      @model={{B.data.name}}
                    >
                      View Policy Definition
                    </LinkTo>
                  </B.Td>
                </B.Tr>
              </:body>
            </HdsTable>
          {{else}}
            <div class="empty-message">
              <h3
                data-test-empty-role-list-headline
                class="empty-message-headline"
              >
                No Policies
              </h3>
              <p class="empty-message-body">
                Get started by
                <LinkTo @route="administration.policies.new">creating a new
                  policy</LinkTo>
              </p>
            </div>
          {{/if}}
        </div>

        <div data-test-token-roles>
          <label>
            Roles
          </label>
          {{#if @roles.length}}
            <HdsTable
              @caption="A list of roles available to this token"
              class="acl-table"
              @model={{@roles}}
              @columns={{array
                (hash key="selected" width="80px")
                (hash key="name" label="Name" isSortable=true)
                (hash key="description" label="Description")
                (hash key="policies" label="Policies")
                (hash key="definition" label="View Role Info")
              }}
              @sortBy="name"
            >
              <:body as |B|>
                <B.Tr>
                  <B.Td class="selection-checkbox">
                    <label>
                      <input
                        type="checkbox"
                        checked={{findBy "name" B.data.name @token.roles}}
                        {{on "change" (fn this.updateTokenRoles B.data)}}
                      />
                    </label>
                  </B.Td>
                  <B.Td data-test-role-name>{{B.data.name}}</B.Td>
                  <B.Td>{{B.data.description}}</B.Td>
                  <B.Td>
                    <div class="tag-group">
                      {{#each B.data.policies as |policy|}}
                        {{#if policy.name}}
                          <HdsTag
                            @color="primary"
                            @text={{policy.name}}
                            @route="administration.policies.policy"
                            @model={{policy.name}}
                          />
                        {{/if}}
                      {{else}}
                        Role contains no policies
                      {{/each}}
                    </div>
                  </B.Td>
                  <B.Td>
                    <LinkTo
                      @route="administration.roles.role"
                      @model={{B.data.id}}
                    >
                      View Role Info
                    </LinkTo>
                  </B.Td>
                </B.Tr>
              </:body>
            </HdsTable>
          {{else}}
            <div class="empty-message">
              <h3
                data-test-empty-role-list-headline
                class="empty-message-headline"
              >
                No Roles
              </h3>
              <p class="empty-message-body">
                Get started by
                <LinkTo @route="administration.roles.new">creating a new role</LinkTo>
              </p>
            </div>
          {{/if}}
        </div>
      {{else}}
        <p>Management-type tokens have access to all permissions.</p>
      {{/if}}

      <footer>
        {{#if (can "update token")}}
          <HdsButton
            @text="Save Token"
            @color="primary"
            type="submit"
            data-test-token-save
          />
        {{/if}}
      </footer>
    </form>
  </template>
}
