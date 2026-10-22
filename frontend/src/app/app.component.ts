import { DatePipe } from '@angular/common';
import { Component, OnInit, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Task, TaskInput, TaskStatus } from './task.model';
import { TaskService } from './task.service';

@Component({
  selector: 'app-root',
  imports: [DatePipe, FormsModule],
  template: `
    <div class="shell">
      <header class="topbar">
        <a class="brand" href="/" aria-label="OpsBoard home"><span class="brand-mark">O</span><span>OpsBoard</span></a>
        <div class="topbar-right"><span class="status-dot"></span><span>Workspace operational</span><span class="avatar">OB</span></div>
      </header>

      <main>
        <section class="welcome">
          <div>
            <p class="eyebrow">WORKSPACE / OVERVIEW</p>
            <h1>Make work visible.</h1>
            <p class="subtitle">A focused view of the work moving your team forward.</p>
          </div>
          <button class="primary" (click)="showForm.set(!showForm())">{{ showForm() ? 'Cancel' : '+ New task' }}</button>
        </section>

        <section class="stats" aria-label="Task summary">
          <article class="stat"><span class="stat-label">Total tasks</span><strong>{{ tasks().length }}</strong><span class="stat-note">Across this workspace</span></article>
          <article class="stat"><span class="stat-label">To do</span><strong>{{ count('todo') }}</strong><span class="stat-note">Ready to be picked up</span></article>
          <article class="stat"><span class="stat-label">In progress</span><strong>{{ count('in_progress') }}</strong><span class="stat-note">Work underway</span></article>
          <article class="stat"><span class="stat-label">Completed</span><strong>{{ count('done') }}</strong><span class="stat-note">Across the finish line</span></article>
        </section>

        @if (showForm()) {
          <section class="form-card">
            <h2>Create a task</h2>
            <form (ngSubmit)="createTask()">
              <label>Task title<input name="title" [(ngModel)]="draft.title" required maxlength="200" placeholder="e.g. Configure deployment pipeline"></label>
              <label>Description<textarea name="description" [(ngModel)]="draft.description" maxlength="5000" rows="3" placeholder="What needs to be done?"></textarea></label>
              <label>Status<select name="status" [(ngModel)]="draft.status"><option value="todo">To do</option><option value="in_progress">In progress</option><option value="done">Done</option></select></label>
              <button class="primary" type="submit" [disabled]="busy() || !draft.title.trim()">Create task</button>
            </form>
          </section>
        }

        @if (error()) {
          <div class="message error" role="alert">{{ error() }} <button (click)="load()">Retry</button></div>
        }
        @if (loading()) {
          <div class="message">Loading workspace…</div>
        }

        @if (!loading()) {
          <section class="board">
            <div class="board-heading"><div><h2>Task board</h2><p>Track tasks from idea to done.</p></div><span class="count-pill">{{ tasks().length }} items</span></div>
            <div class="columns">
              @for (column of columns; track column.status) {
                <section class="column">
                  <div class="column-heading"><span class="column-dot" [class]="column.status"></span><h3>{{ column.label }}</h3><span class="column-count">{{ count(column.status) }}</span></div>
                  @for (task of tasksFor(column.status); track task.id) {
                    <article class="task-card">
                      <div class="task-card-top"><span class="task-id">TASK-{{ task.id }}</span><button class="icon-button" (click)="remove(task)" [disabled]="busy()" [attr.aria-label]="'Delete ' + task.title" title="Delete task">×</button></div>
                      <h4>{{ task.title }}</h4>
                      <p class="task-description">{{ task.description || 'No description provided.' }}</p>
                      <div class="task-footer">
                        <select [ngModel]="task.status" (ngModelChange)="changeStatus(task, $event)" [attr.aria-label]="'Status for ' + task.title" [disabled]="busy()">
                          <option value="todo">To do</option><option value="in_progress">In progress</option><option value="done">Done</option>
                        </select>
                        <span class="task-date">{{ task.updated_at | date:'MMM d' }}</span>
                      </div>
                    </article>
                  } @empty {
                    <div class="empty-column">Nothing here yet</div>
                  }
                </section>
              }
            </div>
          </section>
        }
        <footer><span>OpsBoard</span><span>Built for reliable delivery.</span></footer>
      </main>
    </div>
  `,
  styles: [`
    :host { display:block; min-height:100vh; }
    .shell { min-height:100vh; }
    .topbar { height:70px; display:flex; align-items:center; justify-content:space-between; padding:0 max(24px, calc((100% - 1240px)/2)); background:#101827; color:#f8fafc; }
    .brand { display:flex; gap:11px; align-items:center; color:inherit; text-decoration:none; font-weight:750; letter-spacing:-.4px; font-size:18px; }
    .brand-mark { display:grid; place-items:center; width:30px; height:30px; border-radius:9px; background:#6d5dfc; color:white; font-size:16px; }
    .topbar-right { display:flex; align-items:center; gap:9px; color:#b9c3d4; font-size:12px; }
    .status-dot { width:7px; height:7px; border-radius:50%; background:#39d98a; }
    .avatar { margin-left:14px; display:grid; place-items:center; width:32px; height:32px; border-radius:50%; background:#29364b; color:#fff; font-size:11px; font-weight:700; }
    main { max-width:1240px; margin:0 auto; padding:48px 24px 24px; }
    .welcome { display:flex; align-items:center; justify-content:space-between; gap:20px; margin-bottom:30px; }
    .eyebrow { font-size:10px; letter-spacing:1.6px; font-weight:800; color:#7773e8; margin:0 0 12px; }
    h1 { font-size:clamp(30px,4vw,42px); letter-spacing:-1.7px; margin:0; color:#141d30; }
    .subtitle { margin:10px 0 0; color:#7a8498; font-size:14px; }
    button { border:0; border-radius:9px; font-weight:650; }
    .primary { background:#6558e8; color:white; padding:12px 17px; box-shadow:0 5px 14px #6558e82a; }
    .primary:hover { background:#5144d4; }
    .stats { display:grid; grid-template-columns:repeat(4,1fr); gap:15px; margin-bottom:34px; }
    .stat { background:white; border:1px solid #e9ecf4; border-radius:13px; padding:20px; box-shadow:0 4px 15px #17203305; }
    .stat-label,.stat-note { display:block; color:#7b8497; font-size:12px; }
    .stat strong { display:block; font-size:30px; letter-spacing:-1px; margin:12px 0 6px; color:#182237; }
    .stat-note { font-size:11px; }
    .board { background:#edf0f7; border:1px solid #e5e9f2; border-radius:16px; padding:23px; }
    .board-heading { display:flex; align-items:center; justify-content:space-between; margin-bottom:22px; }
    h2 { margin:0; font-size:18px; letter-spacing:-.4px; color:#1a2438; }
    .board-heading p { margin:6px 0 0; color:#8790a2; font-size:12px; }
    .count-pill,.column-count { padding:5px 9px; border-radius:20px; background:#fff; color:#6f7890; font-size:11px; }
    .columns { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:15px; align-items:start; }
    .column { min-width:0; }
    .column-heading { display:flex; align-items:center; gap:9px; padding:0 2px 13px; }
    .column-heading h3 { font-size:12px; margin:0; color:#48536a; font-weight:750; }
    .column-dot { width:8px; height:8px; border-radius:50%; background:#929db1; }
    .column-dot.in_progress { background:#f2a83b; }.column-dot.done { background:#2fbe87; }
    .column-count { margin-left:auto; padding:3px 7px; background:#e1e5ef; }
    .task-card { background:white; border:1px solid #e4e8f1; border-radius:11px; padding:15px; margin-bottom:11px; box-shadow:0 3px 10px #15213b05; }
    .task-card-top { display:flex; align-items:center; justify-content:space-between; }
    .task-id { color:#8c93a5; font-size:9px; letter-spacing:.8px; font-weight:750; }
    .icon-button { background:transparent; color:#9aa2b1; font-size:20px; line-height:1; padding:1px 4px; }
    .icon-button:hover { color:#d14e61; }
    h4 { margin:9px 0 7px; font-size:13px; line-height:1.5; color:#273148; }
    .task-description { color:#8992a4; font-size:11px; line-height:1.6; min-height:34px; margin:0 0 15px; overflow-wrap:anywhere; }
    .task-footer { display:flex; align-items:center; justify-content:space-between; gap:8px; border-top:1px solid #f0f2f7; padding-top:11px; }
    .task-footer select { max-width:130px; border:1px solid #e7eaf2; background:#fafbfe; border-radius:6px; padding:5px; color:#68748a; font-size:10px; }
    .task-date { color:#a0a7b5; font-size:10px; }
    .empty-column { padding:24px 8px; text-align:center; border:1px dashed #d2d8e5; border-radius:10px; color:#98a0b0; font-size:11px; }
    .form-card { background:white; border:1px solid #e5e9f2; border-radius:14px; padding:22px; margin-bottom:24px; }
    form { display:grid; grid-template-columns:1fr 1.5fr 180px auto; align-items:end; gap:13px; margin-top:18px; }
    label { display:grid; gap:7px; color:#606b80; font-size:11px; font-weight:700; }
    input,textarea,form select { width:100%; border:1px solid #dfe4ee; border-radius:8px; padding:10px; color:#253047; background:#fff; font-size:12px; }
    .message { padding:16px; background:#fff; border-radius:10px; color:#737e93; margin-bottom:18px; font-size:13px; }
    .error { color:#b42335; border:1px solid #ffd5db; }.error button { padding:6px 10px; margin-left:8px; background:#ffe9ed; color:#a61c2d; }
    footer { display:flex; justify-content:space-between; padding:24px 2px; color:#9aa2b2; font-size:10px; }
    @media(max-width:900px) { .stats { grid-template-columns:repeat(2,1fr); } .columns { grid-template-columns:1fr; } form { grid-template-columns:1fr 1fr; } }
    @media(max-width:560px) { main { padding:30px 14px 15px; } .topbar { padding:0 14px; } .topbar-right { font-size:0; } .welcome { align-items:flex-start; flex-direction:column; } .stats { gap:9px; } .stat { padding:14px; } .stat strong { font-size:25px; } .board { padding:14px; } form { grid-template-columns:1fr; } }
  `]
})
export class AppComponent implements OnInit {
  private readonly service = inject(TaskService);

  // State lives in signals: components are OnPush by default in Angular 22, so
  // plain field mutations made in async callbacks would not update the view.
  readonly tasks = signal<Task[]>([]);
  readonly loading = signal(true);
  readonly busy = signal(false);
  readonly showForm = signal(false);
  readonly error = signal('');

  draft: TaskInput = { title: '', description: '', status: 'todo' };
  readonly columns: { label: string; status: TaskStatus }[] = [
    { label: 'To do', status: 'todo' },
    { label: 'In progress', status: 'in_progress' },
    { label: 'Completed', status: 'done' }
  ];

  ngOnInit(): void { this.load(); }

  load(): void {
    this.loading.set(true);
    this.error.set('');
    this.service.list().subscribe({
      next: tasks => { this.tasks.set(tasks); this.loading.set(false); },
      error: () => { this.error.set('Could not load tasks. Check that the API and database are available.'); this.loading.set(false); }
    });
  }

  count(status: TaskStatus): number { return this.tasksFor(status).length; }
  tasksFor(status: TaskStatus): Task[] { return this.tasks().filter(task => task.status === status); }

  createTask(): void {
    if (!this.draft.title.trim() || this.busy()) return;
    this.busy.set(true);
    this.error.set('');
    this.service.create({ ...this.draft, title: this.draft.title.trim() }).subscribe({
      next: task => {
        this.tasks.update(list => [task, ...list]);
        this.draft = { title: '', description: '', status: 'todo' };
        this.showForm.set(false);
        this.busy.set(false);
      },
      error: () => { this.error.set('Task could not be created. Please try again.'); this.busy.set(false); }
    });
  }

  changeStatus(task: Task, status: TaskStatus): void {
    this.busy.set(true);
    this.error.set('');
    this.service.updateStatus(task.id, status).subscribe({
      next: updated => {
        this.tasks.update(list => list.map(item => item.id === updated.id ? updated : item));
        this.busy.set(false);
      },
      error: () => { this.error.set('Status update failed. Refresh and try again.'); this.busy.set(false); }
    });
  }

  remove(task: Task): void {
    if (this.busy() || !confirm(`Delete "${task.title}"? This cannot be undone.`)) return;
    this.busy.set(true);
    this.error.set('');
    this.service.delete(task.id).subscribe({
      next: () => {
        this.tasks.update(list => list.filter(item => item.id !== task.id));
        this.busy.set(false);
      },
      error: () => { this.error.set('Task could not be deleted. Please try again.'); this.busy.set(false); }
    });
  }
}
