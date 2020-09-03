import Component from '@glimmer/component';

export default class TopoViz extends Component {
  get count() {
    return this.args.node.get('allocations.length');
  }
}
