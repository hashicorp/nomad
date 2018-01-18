import { find, findAll } from 'ember-native-dom-helpers';
import { test, moduleForComponent } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import cleanWhitespace from '../utils/clean-whitespace';

moduleForComponent('placement-failure', 'Integration | Component | placement failures', {
  integration: true,
});

const commonTemplate = hbs`
  <div class="boxed-section">
    <div class="boxed-section-body">
      {{#placement-failure taskGroup=taskGroup}}{{/placement-failure}}
    </div>
  </div>
`;


test('should render the placement failure (basic render)', function(assert) {
  const name = 'Placement Failure';
  const failures = 11;
  this.set(
    'taskGroup',
    createFixture(
      {
        coalescedFailures: failures - 1
      },
      name
    )
  );
  this.render(commonTemplate);
  assert.equal(
    cleanWhitespace(find('[data-test-placement-failure-task-group]').firstChild.wholeText),
    name,
    'Title is rendered with the name of the placement failure'
  );
  assert.equal(
    parseInt(find('[data-test-placement-failure-coalesced-failures]').textContent),
    failures,
    'Title is rendered correctly with a count of unplaced'
  );
  assert.equal(
    findAll('[data-test-placement-failure-no-evaluated-nodes]').length,
    0,
    'No evaluated nodes message shown'
  );
});
test('should render correctly when a node is not evaluated', function(assert) {
  this.set(
    'taskGroup',
    createFixture(
      {
        nodesEvaluated: 0
      }
    )
  );
  this.render(commonTemplate);
  assert.equal(
    findAll('[data-test-placement-failure-no-evaluated-nodes]').length,
    1,
    'No evaluated nodes message shown'
  );
});

function createFixture(obj = {}, name = "Placement Failure") {
  return {
    name: name,
    placementFailures: Object.assign({
      coalescedFailures: 10,
      nodesEvaluated: 1,
      nodesAvailable: [
        {
          datacenter: 0,
        },
      ],
      classFiltered: [
        {
          filtered: 1,
        },
      ],
      constraintFiltered: [
        {
          'prop = val': 1,
        },
      ],
      nodesExhausted: 3,
      classExhausted: [
        {
          class: 3,
        },
      ],
      dimensionExhausted: [
        {
          iops: 3,
        },
      ],
      quotaExhausted: [
        {
          quota: 'dimension',
        },
      ],
      scores: [
        {
          name: 3,
        },
      ],
    },
    obj
    )
  };
}
