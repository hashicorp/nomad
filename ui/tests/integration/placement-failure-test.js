import { find } from 'ember-native-dom-helpers';
import { test, moduleForComponent } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import cleanWhitespace from '../utils/clean-whitespace';

moduleForComponent('placement-failure', 'Integration | Component | placement failure', {
  integration: true,
});

const commonTemplate = hbs`
  <div class="boxed-section">
    <div class="boxed-section-body">
      {{#placement-failure taskGroup=taskGroup}}{{/placement-failure}}
    </div>
  </div>
`;

const name = 'My Name';
const failures = 10;

test('placement failure report', function(assert) {
  this.set('taskGroup', {
    name: name,
    placementFailures: createFailures(failures),
  });
  this.render(commonTemplate);
  assert.equal(
    cleanWhitespace(find('.title').textContent),
    `${name} ${failures} unplaced`,
    'Title is rendered correctly with a count of unplaced'
  );
});

function createFailures(count) {
  return {
    coalescedFailures: count,
    nodesEvaluated: 0,
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
  };
}
