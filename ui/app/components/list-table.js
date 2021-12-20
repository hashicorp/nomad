import Component from '@glimmer/component';

export default class ListTable extends Component {
  get decoratedSource() {
    return this.args.source.map(row => ({
      model: row,
    }));
  }
}
