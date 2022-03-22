import { assign, createMachine, send } from 'xstate';

export default createMachine(
  {
    id: 'evaluations_ui',
    context: { evaluation: null },
    type: 'parallel',
    states: {
      table: {
        initial: 'unknown',
        on: {
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
          open: {
            initial: 'busy',
            exit: ['removeCurrentEvaluationQueryParameter'],
            states: {
              busy: {
                invoke: {
                  src: 'loadEvaluation',
                  onDone: 'success',
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
                      onError: 'errorRelatedEvaluations',
                    },
                  },
                  successRelatedEvaluations: {},
                  errorRelatedEvaluations: {
                    on: {
                      RETRY: 'busy',
                    },
                  },
                },
              },
              error: {
                entry: assign({ error: (_ctx, event) => event.data }),
                on: {
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
