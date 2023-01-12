import { A } from '@ember/array';
import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class JobsRunTemplatesController extends Controller {
  @service router;
  @tracked templateName = null;
  @tracked templateNamespace = 'default';

  keyValues = A([]);

  get namespaceOptions() {
    const namespaces = this.store
      .peekAll('namespace')
      .map(({ name }) => ({ key: name, label: name }));

    return namespaces;
  }

  @action
  updateKeyValue(key, value) {
    if (this.keyValues.find((kv) => kv.key === key)) {
      this.keyValues.find((kv) => kv.key === key).value = value;
    } else {
      this.keyValues.pushObject({ key, value });
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

    this.model.set('keyValues', this.keyValues);
    this.model.set('path', `nomad/job-templates/${this.templateName}`);
    this.model.setAndTrimPath();

    try {
      await this.model.save({ adapterOptions: { overwrite } });

      this.flashMessages.add({
        title: 'Job template saved',
        message: `${this.templateName} successfully saved`,
        type: 'success',
        destroyOnClick: false,
        timeout: 5000,
      });

      this.router.transitionTo('jobs.run.templates');
    } catch (e) {
      this.flashMessages.add({
        title: 'Job template cannot be saved.',
        message: e,
        type: 'error',
        destroyOnClick: false,
        timeout: 5000,
      });
    }
  }
}
