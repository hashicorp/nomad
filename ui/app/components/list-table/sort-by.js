import Component from '@glimmer/component';

export default class SortBy extends Component {
  get isActive() {
    return this.args.currentProp === this.args.prop;
  }

  get shouldSortDescending() {
    return !this.isActive || !this.args.sortDescending;
  }

  get class() {
    let result = 'is-selectable';

    if (this.isActive) result = result.concat(' is-active');

    result = this.shouldSortDescending ? result.concat(' desc') : result.concat(' asc');

    return result;
  }
}
