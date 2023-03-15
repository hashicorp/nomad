import Component from '@ember/component';
import { assert } from '@ember/debug';
import { inject as service } from '@ember/service';
import { computed, action } from '@ember/object';
import { task } from 'ember-concurrency';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';
import { attributeBindings } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@attributeBindings('data-test-job-editor')
export default class JobEditor extends Component {
  @service store;
  @service config;

  'data-test-job-editor' = true;

  job = null;
  onSubmit() {}

  @computed('_context')
  get context() {
    return this._context;
  }

  set context(value) {
    const allowedValues = ['new', 'edit'];

    assert(
      `context must be one of: ${allowedValues.join(', ')}`,
      allowedValues.includes(value)
    );

    this.set('_context', value);
  }

  @action updateCode(value) {
    if (!this.job.isDestroying && !this.job.isDestroyed) {
      this.job.set('_newDefinition', value);
    }
  }

  _context = null;
  parseError = null;
  planError = null;
  runError = null;

  planOutput = null;

  @localStorageProperty('nomadMessageJobPlan', true) showPlanMessage;

  @computed('planOutput')
  get stage() {
    return this.planOutput ? 'plan' : 'editor';
  }

  @(task(function* () {
    this.reset();

    try {
      yield this.job.parse();
    } catch (err) {
      const error =
        messageFromAdapterError(err, 'parse jobs') || 'Could not parse input';
      this.set('parseError', error);
      this.scrollToError();
      return;
    }

    try {
      const plan = yield this.job.plan();
      this.set('planOutput', plan);
    } catch (err) {
      const error =
        messageFromAdapterError(err, 'plan jobs') || 'Could not plan job';
      this.set('planError', error);
      this.scrollToError();
    }
  }).drop())
  plan;

  @task(function* () {
    try {
      if (this.context === 'new') {
        yield this.job.run();
      } else {
        yield this.job.update();
      }

      const id = this.get('job.plainId');
      const namespace = this.get('job.namespace.name') || 'default';

      this.reset();

      // Treat the job as ephemeral and only provide ID parts.
      this.onSubmit(id, namespace);
    } catch (err) {
      const error = messageFromAdapterError(err) || 'Could not submit job';
      this.set('runError', error);
      this.set('planOutput', null);
      this.scrollToError();
    }
  })
  submit;

  reset() {
    this.set('planOutput', null);
    this.set('planError', null);
    this.set('parseError', null);
    this.set('runError', null);
  }

  scrollToError() {
    if (!this.get('config.isTest')) {
      window.scrollTo(0, 0);
    }
  }

  @action uploadJobSpec(event) {
    const reader = new FileReader();
    reader.onload = () => {
      this.updateCode(reader.result);
    };

    const [file] = event.target.files;
    reader.readAsText(file);
  }
}
