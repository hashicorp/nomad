import Component from '@glimmer/component';

export default class ConditionalLinkToComponent extends Component {
  get query() {
    return this.args.query || {};
  }
}
