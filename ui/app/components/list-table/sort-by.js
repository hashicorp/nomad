import Component from '@ember/component';
import { computed } from '@ember/object';
import {
  classNames,
  attributeBindings,
  classNameBindings,
  tagName,
} from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('th')
@attributeBindings('title')
@classNames('is-selectable')
@classNameBindings('isActive:is-active', 'sortDescending:desc:asc')
export default class SortBy extends Component {
  // The prop that the table is currently sorted by
  currentProp = '';

  // The prop this sorter controls
  prop = '';

  @computed('currentProp', 'prop')
  get isActive() {
    return this.currentProp === this.prop;
  }

  @computed('sortDescending', 'isActive')
  get shouldSortDescending() {
    return !this.isActive || !this.sortDescending;
  }

  @computed('addToQuery', 'prop', 'shouldSortDescending')
  get query() {
    const addToQuery = this.addToQuery || {};
    return {
      sortProperty: this.prop,
      sortDescending: this.shouldSortDescending,
      ...addToQuery,
    };
  }
}
