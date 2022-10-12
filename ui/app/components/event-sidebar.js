import Component from '@glimmer/component';

export default class EventSidebarComponent extends Component {
  get isSideBarOpen() {
    return !!this.args.event;
  }
}
