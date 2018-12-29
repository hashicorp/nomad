import { run } from '@ember/runloop';
import { getOwner } from '@ember/application';
import { test, moduleForComponent } from 'ember-qunit';
import { click, find, findAll } from 'ember-native-dom-helpers';
import wait from 'ember-test-helpers/wait';
import hbs from 'htmlbars-inline-precompile';
import sinon from 'sinon';
import { clickTrigger } from 'ember-power-select/test-support/helpers';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';

moduleForComponent('job-page/parts/body', 'Integration | Component | job-page/parts/body', {
  integration: true,
  beforeEach() {
    window.localStorage.clear();
    this.server = startMirage();
    this.server.createList('namespace', 3);
  },
  afterEach() {
    this.server.shutdown();
    window.localStorage.clear();
  },
});

test('includes a subnav for the job', function(assert) {
  this.set('job', {});
  this.set('onNamespaceChange', () => {});

  this.render(hbs`
    {{#job-page/parts/body job=job onNamespaceChange=onNamespaceChange}}
      <div class="inner-content">Inner content</div>
    {{/job-page/parts/body}}
  `);

  return wait().then(() => {
    assert.ok(find('[data-test-subnav="job"]'), 'Job subnav is rendered');
  });
});

test('the subnav includes the deployments link when the job is a service', function(assert) {
  const store = getOwner(this).lookup('service:store');
  let job;

  run(() => {
    job = store.createRecord('job', {
      id: 'service-job',
      type: 'service',
    });
  });

  this.set('job', job);
  this.set('onNamespaceChange', () => {});

  this.render(hbs`
    {{#job-page/parts/body job=job onNamespaceChange=onNamespaceChange}}
      <div class="inner-content">Inner content</div>
    {{/job-page/parts/body}}
  `);

  return wait().then(() => {
    const subnavLabels = findAll('[data-test-tab]').map(anchor => anchor.textContent);
    assert.ok(subnavLabels.some(label => label === 'Definition'), 'Definition link');
    assert.ok(subnavLabels.some(label => label === 'Versions'), 'Versions link');
    assert.ok(subnavLabels.some(label => label === 'Deployments'), 'Deployments link');
  });
});

test('the subnav does not include the deployments link when the job is not a service', function(assert) {
  const store = getOwner(this).lookup('service:store');
  let job;

  run(() => {
    job = store.createRecord('job', {
      id: 'batch-job',
      type: 'batch',
    });
  });

  this.set('job', job);
  this.set('onNamespaceChange', () => {});

  this.render(hbs`
    {{#job-page/parts/body job=job onNamespaceChange=onNamespaceChange}}
      <div class="inner-content">Inner content</div>
    {{/job-page/parts/body}}
  `);

  return wait().then(() => {
    const subnavLabels = findAll('[data-test-tab]').map(anchor => anchor.textContent);
    assert.ok(subnavLabels.some(label => label === 'Definition'), 'Definition link');
    assert.ok(subnavLabels.some(label => label === 'Versions'), 'Versions link');
    assert.notOk(subnavLabels.some(label => label === 'Deployments'), 'Deployments link');
  });
});

test('body yields content to a section after the subnav', function(assert) {
  this.set('job', {});
  this.set('onNamespaceChange', () => {});

  this.render(hbs`
    {{#job-page/parts/body job=job onNamespaceChange=onNamespaceChange}}
      <div class="inner-content">Inner content</div>
    {{/job-page/parts/body}}
  `);

  return wait().then(() => {
    assert.ok(
      find('[data-test-page-content] .section > .inner-content'),
      'Content is rendered in a section in a gutter menu'
    );
    assert.ok(
      find('[data-test-subnav="job"] + .section > .inner-content'),
      'Content is rendered immediately after the subnav'
    );
  });
});

test('onNamespaceChange action is called when the namespace changes in the nested gutter menu', function(assert) {
  const namespaceSpy = sinon.spy();

  this.set('job', {});
  this.set('onNamespaceChange', namespaceSpy);

  this.render(hbs`
    {{#job-page/parts/body job=job onNamespaceChange=onNamespaceChange}}
      <div class="inner-content">Inner content</div>
    {{/job-page/parts/body}}
  `);

  return wait().then(() => {
    clickTrigger('[data-test-namespace-switcher]');
    click(findAll('.ember-power-select-option')[1]);

    return wait().then(() => {
      assert.ok(namespaceSpy.calledOnce, 'Switching namespaces calls the onNamespaceChange action');
    });
  });
});
