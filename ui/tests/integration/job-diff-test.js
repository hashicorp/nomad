import { findAll, find } from 'ember-native-dom-helpers';
import { test, moduleForComponent } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('job-diff', 'Integration | Component | job diff', {
  integration: true,
});

const commonTemplate = hbs`
  <div class="boxed-section">
    <div class="boxed-section-body is-dark">
      {{job-diff diff=diff}}
    </div>
  </div>
`;

test('job field diffs', function(assert) {
  this.set('diff', {
    ID: 'test-case-1',
    Type: 'Edited',
    Objects: null,
    Fields: [
      field('Removed Field', 'deleted', 12),
      field('Added Field', 'added', 'Foobar'),
      field('Edited Field', 'edited', 512, 256),
    ],
  });

  this.render(commonTemplate);

  assert.equal(
    findAll('.diff-section-label').length,
    5,
    'A section label for each line, plus one for the group'
  );
  assert.equal(
    cleanWhitespace(find('.diff-section-label .diff-section-label.is-added').textContent),
    '+ Added Field: "Foobar"',
    'Added field is rendered correctly'
  );
  assert.equal(
    cleanWhitespace(find('.diff-section-label .diff-section-label.is-edited').textContent),
    '+/- Edited Field: "256" => "512"',
    'Edited field is rendered correctly'
  );
  assert.equal(
    cleanWhitespace(find('.diff-section-label .diff-section-label.is-deleted').textContent),
    '- Removed Field: "12"',
    'Removed field is rendered correctly'
  );
});

test('job object diffs', function(assert) {
  this.set('diff', {
    ID: 'test-case-2',
    Type: 'Edited',
    Objects: [
      {
        Name: 'ComplexProperty',
        Type: 'Edited',
        Objects: null,
        Fields: [
          field('Prop 1', 'added', 'prop-1-value'),
          field('Prop 2', 'none', 'prop-2-is-the-same'),
          field('Prop 3', 'edited', 'new value', 'some old value'),
          field('Prop 4', 'deleted', 'delete me'),
        ],
      },
      {
        Name: 'DeepConfiguration',
        Type: 'Added',
        Objects: [
          {
            Name: 'VP Props',
            Type: 'Added',
            Objects: null,
            Fields: [
              field('Engineering', 'added', 'Regina Phalange'),
              field('Customer Support', 'added', 'Jerome Hendricks'),
              field('HR', 'added', 'Jack Blue'),
              field('Sales', 'added', 'Maria Lopez'),
            ],
          },
        ],
        Fields: [field('Executive Prop', 'added', 'in charge')],
      },
      {
        Name: 'DatedStuff',
        Type: 'Deleted',
        Objects: null,
        Fields: [field('Deprecated', 'deleted', 'useless')],
      },
    ],
    Fields: null,
  });

  this.render(commonTemplate);

  assert.ok(
    cleanWhitespace(
      find('.diff-section-label > .diff-section-label.is-added').textContent
    ).startsWith('+ DeepConfiguration {'),
    'Added object starts with a JSON block'
  );
  assert.ok(
    cleanWhitespace(
      find('.diff-section-label > .diff-section-label.is-edited').textContent
    ).startsWith('+/- ComplexProperty {'),
    'Edited object starts with a JSON block'
  );
  assert.ok(
    cleanWhitespace(
      find('.diff-section-label > .diff-section-label.is-deleted').textContent
    ).startsWith('- DatedStuff {'),
    'Removed object starts with a JSON block'
  );

  assert.ok(
    cleanWhitespace(
      find('.diff-section-label > .diff-section-label.is-added').textContent
    ).endsWith('}'),
    'Added object ends the JSON block'
  );
  assert.ok(
    cleanWhitespace(
      find('.diff-section-label > .diff-section-label.is-edited').textContent
    ).endsWith('}'),
    'Edited object starts with a JSON block'
  );
  assert.ok(
    cleanWhitespace(
      find('.diff-section-label > .diff-section-label.is-deleted').textContent
    ).endsWith('}'),
    'Removed object ends the JSON block'
  );

  assert.equal(
    findAll('.diff-section-label > .diff-section-label.is-added > .diff-section-label').length,
    this.get('diff').Objects[1].Objects.length + this.get('diff').Objects[1].Fields.length,
    'Edited block contains each nested field and object'
  );

  assert.equal(
    findAll(
      '.diff-section-label > .diff-section-label.is-added > .diff-section-label > .diff-section-label .diff-section-table-row'
    ).length,
    this.get('diff').Objects[1].Objects[0].Fields.length,
    'Objects within objects are rendered'
  );
});

function field(name, type, newVal, oldVal) {
  switch (type) {
    case 'added':
      return {
        Annotations: null,
        New: newVal,
        Old: '',
        Type: 'Added',
        Name: name,
      };
    case 'deleted':
      return {
        Annotations: null,
        New: '',
        Old: newVal,
        Type: 'Deleted',
        Name: name,
      };
    case 'edited':
      return {
        Annotations: null,
        New: newVal,
        Old: oldVal,
        Type: 'Edited',
        Name: name,
      };
  }
  return {
    Annotations: null,
    New: newVal,
    Old: oldVal,
    Type: 'None',
    Name: name,
  };
}

function cleanWhitespace(string) {
  return string
    .replace(/\n/g, '')
    .replace(/ +/g, ' ')
    .trim();
}
