import Component from '@ember/component';
import { tagName } from '@ember-decorators/component';
import { computed, get } from '@ember/object';
import { alias } from '@ember/object/computed';

@tagName('')
export default class GlobalSearchMatch extends Component {
  @alias('match.fuzzySearchMatches.firstObject') firstMatch;
  @alias('firstMatch.indices.firstObject') firstIndices;

  @computed('match.name')
  get label() {
    return get(this, 'match.name') || '';
  }

  @computed('label', 'firstIndices.[]')
  get beforeHighlighted() {
    return this.label.substring(0, this.firstIndices[0]);
  }

  @computed('label', 'firstIndices.[]')
  get highlighted() {
    return this.label.substring(this.firstIndices[0], this.firstIndices[1] + 1);
  }

  @computed('label', 'firstIndices.[]')
  get afterHighlighted() {
    return this.label.substring(this.firstIndices[1] + 1);
  }
}
