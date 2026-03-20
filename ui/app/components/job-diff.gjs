/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { dasherize } from '@ember/string';
import JobDiffFieldsAndObjects from 'nomad-ui/components/job-diff-fields-and-objects';

export default class JobDiff extends Component {
  get verbose() {
    return this.args.verbose ?? true;
  }

  lowerType = (item) => (item?.Type || '').toLowerCase();

  marker = (item) => {
    const type = this.lowerType(item);
    if (type === 'added') return '+';
    if (type === 'deleted') return '-';
    if (type === 'edited') return '+/-';
    return '';
  };

  markerClass = (item) => {
    const type = this.lowerType(item);
    return type ? `is-${type}` : '';
  };

  sectionClass = (item) => {
    const type = this.lowerType(item);
    return type ? `diff-section-label is-${type}` : 'diff-section-label';
  };

  isType = (item, type) => this.lowerType(item) === type;

  shouldShowDiff = (item) => this.verbose || this.isType(item, 'edited');

  cssClass = (value) => dasherize(String(value || '').replace(/\//g, '-'));

  isLastAnnotation = (task, index) =>
    index === (task?.Annotations?.length || 0) - 1;

  get rootClass() {
    const classes = ['job-diff'];
    if (this.isType(this.args.diff, 'edited')) classes.push('is-edited');
    if (this.isType(this.args.diff, 'added')) classes.push('is-added');
    if (this.isType(this.args.diff, 'deleted')) classes.push('is-deleted');
    return classes.join(' ');
  }

  <template>
    <div class={{this.rootClass}}>
      <div
        data-test-diff-section-label="job"
        data-test-diff-field={{this.lowerType @diff}}
        class={{this.sectionClass @diff}}
      >
        <span class="marker {{this.markerClass @diff}}">
          {{this.marker @diff}}
        </span>
        <span class="diff-section-bold">Job: "{{@diff.ID}}"</span>
      </div>

      {{#if (this.shouldShowDiff @diff)}}
        <div data-test-diff-section-label="job-diff" class="diff-section-label">
          <JobDiffFieldsAndObjects
            @fields={{@diff.Fields}}
            @objects={{@diff.Objects}}
          />
        </div>
      {{/if}}

      {{#each @diff.TaskGroups as |group|}}
        <div
          data-test-diff-section-label="task-group"
          class={{this.sectionClass group}}
        >
          <span class="marker {{this.markerClass group}}">
            {{this.marker group}}
          </span>
          <span class="diff-section-bold">Task Group: "{{group.Name}}"</span>
          {{#if group.Updates}}
            ({{#each-in group.Updates as |updateType count|}}
              <span
                class="job-diff-update-count {{this.cssClass updateType}}"
              >{{count}}
                {{updateType}}</span>
            {{/each-in}})
          {{/if}}

          {{#if (this.shouldShowDiff group)}}
            <div
              data-test-diff-section-label="task-group-diff"
              class="diff-section-label"
            >
              <JobDiffFieldsAndObjects
                @fields={{group.Fields}}
                @objects={{group.Objects}}
              />
            </div>
          {{/if}}

          {{#each group.Tasks as |task|}}
            <div
              data-test-diff-section-label="task"
              data-test-diff-field={{this.lowerType task}}
              class={{this.sectionClass task}}
            >
              <span class="marker {{this.markerClass task}}">
                {{this.marker task}}
              </span>
              Task: "{{task.Name}}"
              {{#if task.Annotations}}
                ({{~#each task.Annotations as |annotation index|}}
                  <span class={{this.cssClass annotation}}>{{annotation}}</span>
                  {{#unless (this.isLastAnnotation task index)}},{{/unless}}
                {{/each~}})
              {{/if}}
              {{#if (this.shouldShowDiff task)}}
                <JobDiffFieldsAndObjects
                  @fields={{task.Fields}}
                  @objects={{task.Objects}}
                />
              {{/if}}
            </div>
          {{/each}}
        </div>
      {{/each}}
    </div>
  </template>
}
