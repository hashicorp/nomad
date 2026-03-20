/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { array } from '@ember/helper';
import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import onClickOutside from 'ember-click-outside/modifiers/on-click-outside';
import reverse from '@nullvoxpopuli/ember-composable-helpers/helpers/reverse';
import ListTable from 'nomad-ui/components/list-table';
import TaskLog from 'nomad-ui/components/task-log';
import formatTs from 'nomad-ui/helpers/format-ts';
import keyboardCommands from 'nomad-ui/helpers/keyboard-commands';

export default class TaskContextSidebar extends Component {
  @tracked wide = false;

  get isSideBarOpen() {
    return !!this.args.task;
  }

  get portalTargetElement() {
    if (typeof document === 'undefined') {
      return null;
    }

    return document.getElementById('log-sidebar-portal');
  }

  keyCommands = [
    {
      label: 'Close Task Logs Sidebar',
      pattern: ['Escape'],
      action: () => this.args.fns.closeSidebar(),
    },
  ];

  narrowCommand = {
    label: 'Narrow Sidebar',
    pattern: ['ArrowRight', 'ArrowRight'],
    action: () => this.toggleWide(),
  };

  widenCommand = {
    label: 'Widen Sidebar',
    pattern: ['ArrowLeft', 'ArrowLeft'],
    action: () => this.toggleWide(),
  };

  toggleWide = () => {
    this.wide = !this.wide;
  };

  <template>
    {{#if this.portalTargetElement}}
      {{#in-element this.portalTargetElement}}
        <div
          class="sidebar task-context-sidebar has-subnav
            {{if this.wide 'wide'}}
            {{if @task.events.length 'has-events'}}
            {{if this.isSideBarOpen 'open'}}"
          {{onClickOutside @fns.closeSidebar capture=true}}
        >
          {{#if @task}}
            {{keyboardCommands this.keyCommands}}
            <header>
              <h1 class="title">
                {{@task.name}}
                <span class="state {{@task.state}}">
                  {{@task.state}}
                </span>
              </h1>
              <LinkTo
                class="link"
                title={{@task.name}}
                @route="allocations.allocation.task"
                @models={{array @task.allocation @task.name}}
              >
                Go to Task page
              </LinkTo>
              <button
                class="button close is-borderless"
                type="button"
                {{on "click" @fns.closeSidebar}}
              >
                <HdsIcon @name="x" />
              </button>
            </header>
            {{#if @task.events.length}}
              <div class="boxed-section task-events">
                <div class="boxed-section-head">
                  Recent Events
                </div>
                <div class="boxed-section-body is-full-bleed">
                  <ListTable
                    @source={{reverse @task.events}}
                    @class="is-striped"
                    as |t|
                  >
                    <t.head>
                      <th class="is-3">
                        Time
                      </th>
                      <th class="is-1">
                        Type
                      </th>
                      <th>
                        Description
                      </th>
                    </t.head>
                    <t.body as |row|>
                      <tr data-test-task-event>
                        <td data-test-task-event-time>
                          {{formatTs row.model.time}}
                        </td>
                        <td data-test-task-event-type>
                          {{row.model.type}}
                        </td>
                        <td data-test-task-event-message>
                          {{#if row.model.message}}
                            {{row.model.message}}
                          {{else}}
                            <em>
                              No message
                            </em>
                          {{/if}}
                        </td>
                      </tr>
                    </t.body>
                  </ListTable>
                </div>
              </div>
            {{/if}}

            <TaskLog
              @allocation={{@task.allocation}}
              @task={{@task.name}}
              @shouldFillHeight={{false}}
            />

          {{/if}}
          <button
            class="button is-borderless widener"
            type="button"
            {{on "click" this.toggleWide}}
          >
            {{#if this.wide}}
              {{keyboardCommands (array this.narrowCommand)}}
            {{else}}
              {{keyboardCommands (array this.widenCommand)}}
            {{/if}}
            <HdsIcon @name={{if this.wide "arrow-right" "arrow-left"}} />
          </button>
        </div>
      {{/in-element}}
    {{/if}}
  </template>
}
