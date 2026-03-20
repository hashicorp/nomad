/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { service } from '@ember/service';
import didInsert from '@ember/render-modifiers/modifiers/did-insert';
import Trigger from 'nomad-ui/components/trigger';
import formatDuration from 'nomad-ui/utils/format-duration';

const PARAMS_REQUIRING_CONVERSION = [
  'HealthyDeadline',
  'MinHealthyTime',
  'ProgressDeadline',
  'Stagger',
];

export default class JobStatusUpdateParams extends Component {
  @service notifications;

  @tracked rawDefinition = null;

  get updateParamGroups() {
    if (!this.rawDefinition) {
      return null;
    }

    return this.rawDefinition.TaskGroups.map((taskGroup) => ({
      name: taskGroup.Name,
      update: Object.keys(taskGroup.Update || {}).reduce(
        (newUpdateObj, key) => {
          newUpdateObj[key] = PARAMS_REQUIRING_CONVERSION.includes(key)
            ? formatDuration(taskGroup.Update[key])
            : taskGroup.Update[key];
          return newUpdateObj;
        },
        {},
      ),
    }));
  }

  onError = ({ Error }) => {
    const error = Error.errors[0].title || 'Error fetching job parameters';
    this.notifications.add({
      title: 'Could not fetch job definition',
      message: error,
      color: 'critical',
    });
  };

  fetchJobDefinition = async () => {
    this.rawDefinition = await this.args.job.fetchRawDefinition();
  };

  <template>
    <Trigger
      @onError={{this.onError}}
      @do={{this.fetchJobDefinition}}
      as |trigger|
    >
      <span hidden {{didInsert trigger.fns.do}}></span>

      <div class="update-parameters">
        <h4 class="title is-4">Update Params</h4>
        <code>

          {{#if trigger.data.isSuccess}}
            <ul>
              {{#each this.updateParamGroups as |group|}}
                <li>
                  <span class="group">Group "{{group.name}}"</span>
                  <ul>
                    {{#each-in group.update as |key value|}}
                      <li>
                        <span class="key">{{key}}</span>
                        <span class="value">{{value}}</span>
                      </li>
                    {{/each-in}}
                  </ul>
                </li>
              {{/each}}
            </ul>
          {{/if}}

          {{#if trigger.data.isBusy}}
            <span class="notification">Loading Parameters</span>
          {{/if}}

          {{#if trigger.data.isError}}
            <span class="notification">Error loading parameters</span>
          {{/if}}

        </code>
      </div>
    </Trigger>
  </template>
}
