/* eslint-disable ember/no-observers */
/* eslint-disable ember/no-incorrect-calls-with-inline-anonymous-functions */
import { alias } from '@ember/object/computed';
import Controller from '@ember/controller';
import { action, computed } from '@ember/object';
import { observes } from '@ember-decorators/object';
import { scheduleOnce } from '@ember/runloop';
import { task } from 'ember-concurrency';
import intersection from 'lodash.intersection';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';
import {
  serialize,
  deserializedQueryParam as selection,
} from 'nomad-ui/utils/qp-serialize';
import classic from 'ember-classic-decorator';

@classic
export default class EvaluationController extends Controller.extend(
  Sortable,
  Searchable
) {}
