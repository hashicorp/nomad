import { getOwner } from '@ember/application';
import { assign } from '@ember/polyfills';
import { run } from '@ember/runloop';
import { test, moduleForComponent } from 'ember-qunit';
import wait from 'ember-test-helpers/wait';
import hbs from 'htmlbars-inline-precompile';
import { create } from 'ember-cli-page-object';
import sinon from 'sinon';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { getCodeMirrorInstance } from 'nomad-ui/tests/helpers/codemirror';
import jobEditor from 'nomad-ui/tests/pages/components/job-editor';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';

const Editor = create(jobEditor());

moduleForComponent('job-editor', 'Integration | Component | job-editor', {
  integration: true,
  beforeEach() {
    window.localStorage.clear();

    fragmentSerializerInitializer(getOwner(this));

    // Normally getCodeMirrorInstance is a registered test helper,
    // but those registered test helpers only work in acceptance tests.
    window._getCodeMirrorInstance = window.getCodeMirrorInstance;
    window.getCodeMirrorInstance = getCodeMirrorInstance(getOwner(this));

    this.store = getOwner(this).lookup('service:store');
    this.server = startMirage();

    // Required for placing allocations (a result of creating jobs)
    this.server.create('node');

    Editor.setContext(this);
  },
  afterEach() {
    this.server.shutdown();
    Editor.removeContext();
    window.getCodeMirrorInstance = window._getCodeMirrorInstance;
    delete window._getCodeMirrorInstance;
  },
});

const newJobName = 'new-job';
const newJobTaskGroupName = 'redis';
const jsonJob = overrides => {
  return JSON.stringify(
    assign(
      {},
      {
        Name: newJobName,
        Namespace: 'default',
        Datacenters: ['dc1'],
        Priority: 50,
        TaskGroups: [
          {
            Name: newJobTaskGroupName,
            Tasks: [
              {
                Name: 'redis',
                Driver: 'docker',
              },
            ],
          },
        ],
      },
      overrides
    ),
    null,
    2
  );
};

const hclJob = () => `
job "${newJobName}" {
  namespace = "default"
  datacenters = ["dc1"]

  task "${newJobTaskGroupName}" {
    driver = "docker"
  }
}
`;

const commonTemplate = hbs`
  {{job-editor
    job=job
    context=context
    onSubmit=onSubmit}}
`;

const cancelableTemplate = hbs`
  {{job-editor
    job=job
    context=context
    cancelable=true
    onSubmit=onSubmit
    onCancel=onCancel}}
`;

const renderNewJob = (component, job) => () => {
  component.setProperties({ job, onSubmit: sinon.spy(), context: 'new' });
  component.render(commonTemplate);
  return wait();
};

const renderEditJob = (component, job) => () => {
  component.setProperties({ job, onSubmit: sinon.spy(), onCancel: sinon.spy(), context: 'edit' });
  component.render(cancelableTemplate);
};

const planJob = spec => () => {
  Editor.editor.fillIn(spec);
  return wait().then(() => {
    Editor.plan();
    return wait();
  });
};

test('the default state is an editor with an explanation popup', function(assert) {
  let job;
  run(() => {
    job = this.store.createRecord('job');
  });

  return wait()
    .then(renderNewJob(this, job))
    .then(() => {
      assert.ok(Editor.editorHelp.isPresent, 'Editor explanation popup is present');
      assert.ok(Editor.editor.isPresent, 'Editor is present');
    });
});

test('the explanation popup can be dismissed', function(assert) {
  let job;
  run(() => {
    job = this.store.createRecord('job');
  });

  return wait()
    .then(renderNewJob(this, job))
    .then(() => {
      Editor.editorHelp.dismiss();
      return wait();
    })
    .then(() => {
      assert.notOk(Editor.editorHelp.isPresent, 'Editor explanation popup is gone');
      assert.equal(
        window.localStorage.nomadMessageJobEditor,
        'false',
        'Dismissal is persisted in localStorage'
      );
    });
});

test('the explanation popup is not shown once the dismissal state is set in localStorage', function(assert) {
  window.localStorage.nomadMessageJobEditor = 'false';

  let job;
  run(() => {
    job = this.store.createRecord('job');
  });

  return wait()
    .then(renderNewJob(this, job))
    .then(() => {
      assert.notOk(Editor.editorHelp.isPresent, 'Editor explanation popup is gone');
    });
});

test('submitting a json job skips the parse endpoint', function(assert) {
  const spec = jsonJob();
  let job;
  run(() => {
    job = this.store.createRecord('job');
  });

  return wait()
    .then(renderNewJob(this, job))
    .then(planJob(spec))
    .then(() => {
      const requests = this.server.pretender.handledRequests.mapBy('url');
      assert.notOk(requests.includes('/v1/jobs/parse'), 'JSON job spec is not parsed');
      assert.ok(requests.includes(`/v1/job/${newJobName}/plan`), 'JSON job spec is still planned');
    });
});

test('submitting an hcl job requires the parse endpoint', function(assert) {
  const spec = hclJob();
  let job;
  run(() => {
    job = this.store.createRecord('job');
  });

  return wait()
    .then(renderNewJob(this, job))
    .then(planJob(spec))
    .then(() => {
      const requests = this.server.pretender.handledRequests.mapBy('url');
      assert.ok(requests.includes('/v1/jobs/parse'), 'HCL job spec is parsed first');
      assert.ok(requests.includes(`/v1/job/${newJobName}/plan`), 'HCL job spec is planned');
      assert.ok(
        requests.indexOf('/v1/jobs/parse') < requests.indexOf(`/v1/job/${newJobName}/plan`),
        'Parse comes before Plan'
      );
    });
});

test('when a job is successfully parsed and planned, the plan is shown to the user', function(assert) {
  const spec = hclJob();
  let job;
  run(() => {
    job = this.store.createRecord('job');
  });

  return wait()
    .then(renderNewJob(this, job))
    .then(planJob(spec))
    .then(() => {
      assert.ok(Editor.planOutput, 'The plan is outputted');
      assert.notOk(Editor.editor.isPresent, 'The editor is replaced with the plan output');
      assert.ok(Editor.planHelp.isPresent, 'The plan explanation popup is shown');
    });
});

test('from the plan screen, the cancel button goes back to the editor with the job still in tact', function(assert) {
  const spec = hclJob();
  let job;
  run(() => {
    job = this.store.createRecord('job');
  });

  return wait()
    .then(renderNewJob(this, job))
    .then(planJob(spec))
    .then(() => {
      Editor.cancel();
      return wait();
    })
    .then(() => {
      assert.ok(Editor.editor.isPresent, 'The editor is shown again');
      assert.equal(
        Editor.editor.contents,
        spec,
        'The spec that was planned is still in the editor'
      );
    });
});

test('when parse fails, the parse error message is shown', function(assert) {
  const spec = hclJob();
  const errorMessage = 'Parse Failed!! :o';

  let job;
  run(() => {
    job = this.store.createRecord('job');
  });

  this.server.pretender.post('/v1/jobs/parse', () => [400, {}, errorMessage]);

  return wait()
    .then(renderNewJob(this, job))
    .then(planJob(spec))
    .then(() => {
      assert.notOk(Editor.planError.isPresent, 'Plan error is not shown');
      assert.notOk(Editor.runError.isPresent, 'Run error is not shown');

      assert.ok(Editor.parseError.isPresent, 'Parse error is shown');
      assert.equal(
        Editor.parseError.message,
        errorMessage,
        'The error message from the server is shown in the error in the UI'
      );
    });
});

test('when plan fails, the plan error message is shown', function(assert) {
  const spec = hclJob();
  const errorMessage = 'Plan Failed!! :o';

  let job;
  run(() => {
    job = this.store.createRecord('job');
  });

  this.server.pretender.post(`/v1/job/${newJobName}/plan`, () => [400, {}, errorMessage]);

  return wait()
    .then(renderNewJob(this, job))
    .then(planJob(spec))
    .then(() => {
      assert.notOk(Editor.parseError.isPresent, 'Parse error is not shown');
      assert.notOk(Editor.runError.isPresent, 'Run error is not shown');

      assert.ok(Editor.planError.isPresent, 'Plan error is shown');
      assert.equal(
        Editor.planError.message,
        errorMessage,
        'The error message from the server is shown in the error in the UI'
      );
    });
});

test('when run fails, the run error message is shown', function(assert) {
  const spec = hclJob();
  const errorMessage = 'Run Failed!! :o';

  let job;
  run(() => {
    job = this.store.createRecord('job');
  });

  this.server.pretender.post('/v1/jobs', () => [400, {}, errorMessage]);

  return wait()
    .then(renderNewJob(this, job))
    .then(planJob(spec))
    .then(() => {
      Editor.run();
      return wait();
    })
    .then(() => {
      assert.notOk(Editor.planError.isPresent, 'Plan error is not shown');
      assert.notOk(Editor.parseError.isPresent, 'Parse error is not shown');

      assert.ok(Editor.runError.isPresent, 'Run error is shown');
      assert.equal(
        Editor.runError.message,
        errorMessage,
        'The error message from the server is shown in the error in the UI'
      );
    });
});

test('when the scheduler dry-run has warnings, the warnings are shown to the user', function(assert) {
  const spec = jsonJob({ Unschedulable: true });
  let job;
  run(() => {
    job = this.store.createRecord('job');
  });

  return wait()
    .then(renderNewJob(this, job))
    .then(planJob(spec))
    .then(() => {
      assert.ok(
        Editor.dryRunMessage.errored,
        'The scheduler dry-run message is in the warning state'
      );
      assert.notOk(
        Editor.dryRunMessage.succeeded,
        'The success message is not shown in addition to the warning message'
      );
      assert.ok(
        Editor.dryRunMessage.body.includes(newJobTaskGroupName),
        'The scheduler dry-run message includes the warning from send back by the API'
      );
    });
});

test('when the scheduler dry-run has no warnings, a success message is shown to the user', function(assert) {
  const spec = hclJob();
  let job;
  run(() => {
    job = this.store.createRecord('job');
  });

  return wait()
    .then(renderNewJob(this, job))
    .then(planJob(spec))
    .then(() => {
      assert.ok(
        Editor.dryRunMessage.succeeded,
        'The scheduler dry-run message is in the success state'
      );
      assert.notOk(
        Editor.dryRunMessage.errored,
        'The warning message is not shown in addition to the success message'
      );
    });
});

test('when a job is submitted in the edit context, a POST request is made to the update job endpoint', function(assert) {
  const spec = hclJob();
  let job;
  run(() => {
    job = this.store.createRecord('job');
  });

  return wait()
    .then(renderEditJob(this, job))
    .then(planJob(spec))
    .then(() => {
      Editor.run();
    })
    .then(() => {
      const requests = this.server.pretender.handledRequests
        .filterBy('method', 'POST')
        .mapBy('url');
      assert.ok(requests.includes(`/v1/job/${newJobName}`), 'A request was made to job update');
      assert.notOk(requests.includes('/v1/jobs'), 'A request was not made to job create');
    });
});

test('when a job is submitted in the new context, a POST request is made to the create job endpoint', function(assert) {
  const spec = hclJob();
  let job;
  run(() => {
    job = this.store.createRecord('job');
  });

  return wait()
    .then(renderNewJob(this, job))
    .then(planJob(spec))
    .then(() => {
      Editor.run();
    })
    .then(() => {
      const requests = this.server.pretender.handledRequests
        .filterBy('method', 'POST')
        .mapBy('url');
      assert.ok(requests.includes('/v1/jobs'), 'A request was made to job create');
      assert.notOk(
        requests.includes(`/v1/job/${newJobName}`),
        'A request was not made to job update'
      );
    });
});

test('when a job is successfully submitted, the onSubmit hook is called', function(assert) {
  const spec = hclJob();
  let job;
  run(() => {
    job = this.store.createRecord('job');
  });

  return wait()
    .then(renderNewJob(this, job))
    .then(planJob(spec))
    .then(() => {
      Editor.run();
      return wait();
    })
    .then(() => {
      assert.ok(
        this.get('onSubmit').calledWith(newJobName, 'default'),
        'The onSubmit hook was called with the correct arguments'
      );
    });
});

test('when the job-editor cancelable flag is false, there is no cancel button in the header', function(assert) {
  let job;
  run(() => {
    job = this.store.createRecord('job');
  });

  return wait()
    .then(renderNewJob(this, job))
    .then(() => {
      assert.notOk(Editor.cancelEditingIsAvailable, 'No way to cancel editing');
    });
});

test('when the job-editor cancelable flag is true, there is a cancel button in the header', function(assert) {
  let job;
  run(() => {
    job = this.store.createRecord('job');
  });

  return wait()
    .then(renderEditJob(this, job))
    .then(() => {
      assert.ok(Editor.cancelEditingIsAvailable, 'Cancel editing button exists');
    });
});

test('when the job-editor cancel button is clicked, the onCancel hook is called', function(assert) {
  let job;
  run(() => {
    job = this.store.createRecord('job');
  });

  return wait()
    .then(renderEditJob(this, job))
    .then(() => {
      Editor.cancelEditing();
    })
    .then(() => {
      assert.ok(this.get('onCancel').calledOnce, 'The onCancel hook was called');
    });
});
