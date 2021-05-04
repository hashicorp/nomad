import { setupRenderingTest } from 'ember-qunit';
import { module, test } from 'qunit';
import { find, render } from '@ember/test-helpers';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';
import hbs from 'htmlbars-inline-precompile';
import { setupPrimaryMetricMocks, primaryMetric } from './primary-metric';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';

const mockTasks = [
  { task: 'One', reservedCPU: 200, reservedMemory: 500, reservedMemoryMax: 600, cpu: [], memory: [] },
  { task: 'Two', reservedCPU: 100, reservedMemory: 200, reservedMemoryMax: 300, cpu: [], memory: [] },
  { task: 'Three', reservedCPU: 300, reservedMemory: 100, reservedMemoryMax: 200, cpu: [], memory: [] },
];

module('Integration | Component | PrimaryMetric::Task', function(hooks) {
  setupRenderingTest(hooks);
  setupPrimaryMetricMocks(hooks, [...mockTasks]);

  hooks.beforeEach(function() {
    fragmentSerializerInitializer(this.owner);
    this.store = this.owner.lookup('service:store');
    this.server = startMirage();
    this.server.create('namespace');
    this.server.create('node');
    const job = this.server.create('job', {
      groupsCount: 1,
      groupTaskCount: 3,
      createAllocations: false,
    });

    // Update job > group > task names to match mockTasks
    job.taskGroups.models[0].tasks.models.forEach((task, index) => {
      task.update({ name: mockTasks[index].task });
    });

    this.server.create('allocation', { forceRunningClientStatus: true });
  });

  hooks.afterEach(function() {
    this.server.shutdown();
  });

  const template = hbs`
    <PrimaryMetric::Task
      @taskState={{this.resource}}
      @metric={{this.metric}} />
  `;

  const preload = async store => {
    await store.findAll('allocation');
  };

  const findResource = store => store.peekAll('allocation').get('firstObject.states.firstObject');

  test('Must pass an accessibility audit', async function(assert) {
    await preload(this.store);

    const resource = findResource(this.store);
    this.setProperties({ resource, metric: 'cpu' });

    await render(template);
    await componentA11yAudit(this.element, assert);
  });

  test('The soft memory limit shows as an annotation', async function(assert) {
    await preload(this.store);

    const resource = findResource(this.store);
    this.setProperties({ resource, metric: 'memory' });

    await render(template);

    assert.ok(find('[data-test-annotation]'));
    assert.equal(
      find('[data-test-annotation]').textContent.trim(),
      '500 MiB soft limit'
    );
  });

  primaryMetric({
    template,
    preload,
    findResource,
  });
});
