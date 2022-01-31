import { assign, createMachine, send } from 'xstate';

// transitory state - what should happen when we navigate away
// do we need a transition

// is this an async request (opening the modal)

// related evaluations
// how do we get the related evaluation statuses
// and how do we get the related evaluations in the chain
// beyond the first set of evaluations

export default createMachine(
  {
    id: 'evaluations_ui',
    context: { evaluation: null },
    type: 'parallel',
    states: {
      // what is the table interaction when the sidebar is open
      table: {
        initial: 'unknown',
        on: {
          // IS this guarded, yes it is -- model this

          // these can be updated in the URL directly, without browser interaction
          NEXT: {
            actions: ['requestNextPage', send('MODAL_CLOSE')],
          },
          PREV: {
            actions: ['requestPrevPage', send('MODAL_CLOSE')],
          },
          CHANGE_PAGES_SIZE: {
            actions: ['changePageSize', send('MODAL_CLOSE')],
          },
          MODEL_UPDATED: '#unknown',
        },
        states: {
          unknown: {
            id: 'unknown',
            always: [{ target: 'data', cond: 'hasData' }, { target: 'empty' }],
          },
          data: {},
          empty: {},
        },
      },
      sidebar: {
        initial: 'unknown',
        states: {
          unknown: {
            always: [
              { target: 'open', cond: 'sidebarIsOpen' },
              { target: 'close' },
            ],
          },
          // ANSWER:  yes -- is this addressable state -- pass URL and detail view is open
          open: {
            // there is no design for the loading state
            initial: 'busy',
            exit: ['removeCurrentEvaluationQueryParameter'],
            states: {
              busy: {
                invoke: {
                  src: 'loadEvaluation',
                  onDone: 'success',
                  // there is no design for error state
                  // should we retry... garbage collection message
                  // or the request times out
                  onError: 'error',
                },
              },
              success: {
                entry: assign({
                  evaluation: (context, event) => {
                    return event.data;
                  },
                }),
                initial: 'busy',
                on: {
                  LOAD_EVALUATION: {
                    target: 'busy',
                    actions: ['updateEvaluationQueryParameter'],
                  },
                },
                states: {
                  busy: {
                    invoke: {
                      src: 'loadRelatedEvaluations',
                      onDone: 'successRelatedEvaluations',
                      // there is no design for error state
                      // should we retry... garbage collection message
                      // or the request times out
                      onError: 'errorRelatedEvaluations',
                    },
                  },
                  successRelatedEvaluations: {},
                  errorRelatedEvaluations: {
                    on: {
                      // should this be capped
                      RETRY: 'busy',
                    },
                  },
                },
              },
              error: {
                on: {
                  // should this be capped
                  RETRY: 'busy',
                },
              },
            },
            on: {
              MODAL_CLOSE: 'close',
            },
          },
          close: {
            on: {
              LOAD_EVALUATION: {
                target: 'open',
                actions: ['updateEvaluationQueryParameter'],
              },
            },
          },
        },
      },
    },
  },
  {
    services: {
      async loadEvaluations() {
        return;
      },
      async loadEvaluation() {},
      // do we do previous and next
      async loadRelatedEvaluations() {
        return;
      },
    },
    guards: {
      sidebarIsOpen() {
        return false;
      },
      hasData() {
        return true;
      },
    },
    actions: {
      updateEvaluationQueryParameter() {},
      removeCurrentEvaluationQueryParameter() {},
    },
  }
);
