import Component from '@glimmer/component';

export default class AppBreadcrumbsComponent extends Component {
  isOneCrumbUp(iter = 0, totalNum = 0) {
    return iter === totalNum - 2;
  }
}
