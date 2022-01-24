import Component from '@ember/component';
import {
  classNames,
  classNameBindings,
  tagName,
} from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('label')
@classNames('toggle')
@classNameBindings('isDisabled:is-disabled', 'isActive:is-active')
export default class Toggle extends Component {
  'data-test-label' = true;

  isActive = false;
  isDisabled = false;
  onToggle() {}
}
