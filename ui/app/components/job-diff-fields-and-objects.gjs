/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

export default class JobDiffFieldsAndObjects extends Component {
  lowerType = (item) => (item?.Type || '').toLowerCase();

  objectTestField = (object) =>
    this.args.nested ? null : this.lowerType(object);

  get fields() {
    return this.args.fields || [];
  }

  get objects() {
    return this.args.objects || [];
  }

  fieldsFor = (object) => object?.Fields || object?.fields || [];

  objectsFor = (object) => object?.Objects || object?.objects || [];

  marker = (item) => {
    const type = this.lowerType(item);
    if (type === 'added') return '+';
    if (type === 'deleted') return '-';
    if (type === 'edited') return '+/-';
    return '';
  };

  sectionClass = (item) => `diff-section-label is-${this.lowerType(item)}`;

  markerClass = (item) => `is-${this.lowerType(item)}`;

  isType = (item, type) => this.lowerType(item) === type;

  <template>
    {{#each this.fields as |field|}}
      <div
        data-test-diff-section-label="field"
        data-test-diff-field={{this.lowerType field}}
        class="diff-section-table-row {{this.sectionClass field}}"
      >
        <span class="diff-section-table-cell">
          <span class="marker {{this.markerClass field}}">
            {{this.marker field}}
          </span>
          {{field.Name}}:
        </span>
        {{#if (this.isType field "added")}}
          <span
            class="diff-section-table-cell diff-section-change"
          >"{{field.New}}"</span>
        {{else if (this.isType field "deleted")}}
          <span
            class="diff-section-table-cell diff-section-change"
          >"{{field.Old}}"</span>
        {{else if (this.isType field "edited")}}
          <span
            class="diff-section-table-cell diff-section-change"
          >"{{field.Old}}" => "{{field.New}}"</span>
        {{else}}
          <span class="diff-section-table-cell">"{{field.New}}"</span>
        {{/if}}
      </div>
    {{/each}}

    {{#each this.objects as |object|}}
      <div
        data-test-diff-section-label="object"
        data-test-diff-field={{this.objectTestField object}}
        class={{this.sectionClass object}}
      >
        <span class="marker {{this.markerClass object}}">
          {{this.marker object}}
        </span>
        {{object.Name}}
        {
        <JobDiffFieldsAndObjects
          @fields={{this.fieldsFor object}}
          @objects={{this.objectsFor object}}
          @nested={{true}}
        />
        }
      </div>
    {{/each}}
  </template>
}
