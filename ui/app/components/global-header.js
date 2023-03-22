import Component from '@ember/component';
import classic from 'ember-classic-decorator';
import { inject as service } from '@ember/service';
import { attributeBindings } from '@ember-decorators/component';

@classic
@attributeBindings('data-test-global-header')
export default class GlobalHeader extends Component {
  @service config;
  @service system;

  'data-test-global-header' = true;
  onHamburgerClick() {}
}
