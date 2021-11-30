import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

export default class Breadcrumbs extends Component {
  @service bucket;

  get crumbs() {
    return this.bucket.crumbs;
  }
}
