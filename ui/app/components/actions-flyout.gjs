/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { service } from '@ember/service';
import { on } from '@ember/modifier';
import {
  HdsButton,
  HdsFlyout,
  HdsApplicationState,
} from '@hashicorp/design-system-components/components';
import ActionCard from 'nomad-ui/components/action-card';
import ActionsDropdown from 'nomad-ui/components/actions-dropdown';

export default class ActionsFlyout extends Component {
  @service nomadActions;
  @service router;

  get job() {
    if (this.task) {
      return this.task.taskGroup.job;
    }

    return (
      this.router.currentRouteName.startsWith('jobs.job') &&
      this.router.currentRoute.attributes
    );
  }

  get task() {
    return (
      this.router.currentRouteName.startsWith('allocations.allocation.task') &&
      this.router.currentRoute.attributes.task
    );
  }

  get allocation() {
    return (
      this.args.allocation ||
      (this.task && this.router.currentRoute.attributes.allocation)
    );
  }

  get contextualParent() {
    return this.task || this.job;
  }

  get contextualActions() {
    return this.contextualParent?.actions || [];
  }

  // Group peers together by their peerID
  get actionInstances() {
    const instances = this.nomadActions.actionsQueue;

    const peerIDs = new Set();
    const filteredInstances = [];
    for (const instance of instances) {
      if (!instance.peerID || !peerIDs.has(instance.peerID)) {
        filteredInstances.push(instance);
        peerIDs.add(instance.peerID);
      }
    }

    return filteredInstances;
  }

  <template>
    {{#if this.nomadActions.flyoutActive}}
      <HdsFlyout
        id="actions-flyout"
        @onClose={{this.nomadActions.closeFlyout}}
        @size="large"
        as |Fly|
      >
        <Fly.Header>
          <h3>
            Actions
          </h3>
          {{#if this.contextualActions.length}}
            <ActionsDropdown
              @actions={{this.contextualActions}}
              @allocation={{this.allocation}}
              @context={{if this.task this.task this.job}}
            />
          {{/if}}
          {{#if this.nomadActions.runningActions.length}}
            <HdsButton
              @text="Stop All"
              @color="critical"
              @size="medium"
              {{on "click" this.nomadActions.stopAll}}
            />
          {{/if}}
          {{#if this.nomadActions.finishedActions.length}}
            <HdsButton
              data-test-clear-finished-actions
              @text="Clear Finished Actions"
              @color="secondary"
              @size="medium"
              {{on "click" this.nomadActions.clearFinishedActions}}
            />
          {{/if}}
        </Fly.Header>
        <Fly.Body>
          <ul class="actions-queue">
            {{#each this.actionInstances as |instance|}}
              <ActionCard @instance={{instance}} />
            {{else}}
              <HdsApplicationState as |A|>
                <A.Header @title="No actions in queue" />
                <A.Body
                  @text="Your actions have been manually cleared. To run more, head to a Job or Task page with actions in its Jobspec, and an Actions dropdown will automatically populate."
                />
                <A.Footer as |F|>
                  <F.LinkStandalone
                    @icon="docs-link"
                    @text="Learn more about Actions"
                    @href="https://developer.hashicorp.com/nomad/docs/job-specification/action"
                    @iconPosition="trailing"
                  />
                </A.Footer>
              </HdsApplicationState>
            {{/each}}
          </ul>
        </Fly.Body>
      </HdsFlyout>
    {{/if}}
  </template>
}
