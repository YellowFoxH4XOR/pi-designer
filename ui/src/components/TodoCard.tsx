import type { Todo, TodoStatus } from '../api';
import { Icon } from './Icon';

const statusLabel: Record<TodoStatus, string> = {
  pending: 'Pending', in_progress: 'In progress', completed: 'Completed',
};

/** The agent's working plan, rendered from todo_write snapshots. */
export function TodoCard({ todos, live = false }: { todos: Todo[]; live?: boolean }) {
  if (!todos.length) return null;
  const done = todos.filter((todo) => todo.status === 'completed').length;
  return <section class={`todo-card ${live ? 'todo-card-live' : ''}`} aria-label="Design plan">
    <header class="todo-card-head">
      <h3>Plan · {todos.length} {todos.length === 1 ? 'step' : 'steps'}</h3>
      <span class="todo-count">{done}/{todos.length}</span>
    </header>
    <ol class="todo-list">
      {todos.map((todo, index) => <li key={`${index}-${todo.content}`} class={`todo-item is-${todo.status}`} aria-current={todo.status === 'in_progress' ? 'step' : undefined}>
        <span class="todo-icon" aria-hidden="true">
          {todo.status === 'completed' ? <Icon name="check" size={12}/> : todo.status === 'in_progress' ? <span class="todo-pulse"/> : <span class="todo-dot"/>}
        </span>
        <span class="todo-content">{todo.content}</span>
        <span class="sr-only">{statusLabel[todo.status] || todo.status}</span>
      </li>)}
    </ol>
  </section>;
}
