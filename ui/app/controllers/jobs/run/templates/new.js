/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { trimPath } from '../../../../helpers/trim-path';

export default class JobsRunTemplatesNewController extends Controller {
  @service router;
  @service store;
  @service system;
  @tracked templateName = null;
  @tracked templateNamespace = 'default';
  @service notifications;

  get namespaceOptions() {
    const namespaces = this.store
      .peekAll('namespace')
      .map(({ name }) => ({ key: name, label: name }));

    return namespaces;
  }

  get isDuplicateTemplate() {
    const templates = this.store.peekAll('variable');
    const templateName = trimPath([`nomad/job-templates/${this.templateName}`]);

    return !!templates
      .without(this.model)
      .find(
        (v) => v.path === templateName && v.namespace === this.templateNamespace
      );
  }

  @action
  updateKeyValue(key, value) {
    if (this.model.keyValues.find((kv) => kv.key === key)) {
      this.model.keyValues.find((kv) => kv.key === key).value = value;
    } else {
      this.model.keyValues.pushObject({ key, value });
    }
  }

  @action
  async save(e, overwrite = false) {
    if (e.type === 'submit') {
      e.preventDefault();
    }

    if (this.model?.isNew) {
      if (this.namespaceOptions) {
        this.model.set('namespace', this.templateNamespace);
      } else {
        const [namespace] = this.store.peekAll('namespace').toArray();
        this.model.set('namespace', namespace.id);
      }
    }

    this.model.set('path', `nomad/job-templates/${this.templateName}`);
    this.model.setAndTrimPath();

    try {
      await this.model.save({ adapterOptions: { overwrite } });

      this.notifications.add({
        title: 'Job template saved',
        message: `${this.templateName} successfully saved`,
        color: 'success',
      });

      this.router.transitionTo('jobs.run.templates');
    } catch (e) {
      this.notifications.add({
        title: 'Job template cannot be saved.',
        message: e,
        color: 'critical',
      });
    }
  }
}
