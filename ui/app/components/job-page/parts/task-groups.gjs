/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { fn } from '@ember/helper';
import Component from '@glimmer/component';
import { service } from '@ember/service';
import ListTable from 'nomad-ui/components/list-table';
import TaskGroupRow from 'nomad-ui/components/task-group-row';
import keyboardShortcutModifier from 'nomad-ui/modifiers/keyboard-shortcut';

export default class TaskGroups extends Component {
  @service router;

  get sortedTaskGroups() {
    return sortItems(
      this.args.job?.taskGroups,
      this.args.sortProperty,
      this.args.sortDescending,
    );
  }

  gotoTaskGroup = (taskGroup) => {
    this.router.transitionTo(
      'jobs.job.task-group',
      this.args.job,
      taskGroup.name,
    );
  };

  <template>
    <div class="boxed-section" ...attributes>
      <div class="boxed-section-head">
        Task Groups
      </div>
      <div class="boxed-section-body is-full-bleed">
        <ListTable
          @source={{this.sortedTaskGroups}}
          @sortProperty={{@sortProperty}}
          @sortDescending={{@sortDescending}}
          as |t|
        >
          <t.head>
            <t.sortBy @prop="name">
              Name
            </t.sortBy>
            <t.sortBy @prop="count">
              Count
            </t.sortBy>
            <t.sortBy @prop="queuedOrStartingAllocs" @class="is-3">
              Allocation Status
            </t.sortBy>
            <t.sortBy @prop="volumes.length">
              Volume
            </t.sortBy>
            <t.sortBy @prop="reservedCPU">
              Reserved CPU
            </t.sortBy>
            <t.sortBy @prop="reservedMemory">
              Reserved Memory
            </t.sortBy>
            <t.sortBy @prop="reservedEphemeralDisk">
              Reserved Disk
            </t.sortBy>
          </t.head>
          <t.body as |row|>
            <TaskGroupRow
              data-test-task-group={{row.model.name}}
              @taskGroup={{row.model}}
              @onClick={{fn this.gotoTaskGroup row.model}}
              {{keyboardShortcutModifier
                enumerated=true
                action=(fn this.gotoTaskGroup row.model)
              }}
            />
          </t.body>
        </ListTable>
      </div>
    </div>
  </template>
}

function sortItems(items, sortProperty, sortDescending = true) {
  const normalizedItems = (items?.toArray?.() || items || []).filter(Boolean);

  if (!sortProperty) {
    return normalizedItems;
  }

  const sortedItems = normalizedItems
    .slice()
    .sort((left, right) => compareValues(left, right, sortProperty));

  return sortDescending ? sortedItems.reverse() : sortedItems;
}

function compareValues(left, right, sortProperty) {
  const leftValue = getPathValue(left, sortProperty);
  const rightValue = getPathValue(right, sortProperty);

  if (typeof leftValue === 'string' && typeof rightValue === 'string') {
    return leftValue.localeCompare(rightValue);
  }

  if (leftValue === rightValue) {
    return 0;
  }

  if (leftValue == null) {
    return -1;
  }

  if (rightValue == null) {
    return 1;
  }

  return leftValue > rightValue ? 1 : -1;
}

function getPathValue(item, sortProperty) {
  return sortProperty.split('.').reduce((value, key) => value?.[key], item);
}
