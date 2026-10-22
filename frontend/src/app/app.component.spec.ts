import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { AppComponent } from './app.component';
import { Task, TaskStatus } from './task.model';

const makeTask = (id: number, status: TaskStatus, title = `Task ${id}`): Task => ({
  id,
  title,
  description: '',
  status,
  created_at: '2026-01-01T12:00:00Z',
  updated_at: '2026-01-01T12:00:00Z'
});

describe('AppComponent', () => {
  let http: HttpTestingController;
  let fixture: ComponentFixture<AppComponent>;

  const el = () => fixture.nativeElement as HTMLElement;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [AppComponent],
      providers: [provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    http = TestBed.inject(HttpTestingController);
    fixture = TestBed.createComponent(AppComponent);
    fixture.detectChanges();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    http.verify();
  });

  it('creates the component and requests tasks on init', () => {
    expect(fixture.componentInstance).toBeTruthy();
    http.expectOne('/api/v1/tasks').flush([]);
  });

  it('renders tasks and per-status counts', () => {
    http.expectOne('/api/v1/tasks').flush([makeTask(1, 'todo'), makeTask(2, 'done'), makeTask(3, 'done')]);
    fixture.detectChanges();
    expect(el().querySelectorAll('.task-card').length).toBe(3);
    expect(el().textContent).toContain('TASK-1');
    expect(fixture.componentInstance.count('done')).toBe(2);
    expect(el().querySelectorAll('.empty-column').length).toBe(1);
  });

  it('shows an error when loading fails', () => {
    http.expectOne('/api/v1/tasks').flush('boom', { status: 500, statusText: 'Server Error' });
    fixture.detectChanges();
    expect(el().querySelector('.error')?.textContent).toContain('Could not load tasks');
  });

  it('creates a task and resets the draft', () => {
    http.expectOne('/api/v1/tasks').flush([]);
    const cmp = fixture.componentInstance;
    cmp.showForm.set(true);
    cmp.draft.title = '  New task  ';
    cmp.createTask();

    const req = http.expectOne('/api/v1/tasks');
    expect(req.request.method).toBe('POST');
    expect(req.request.body.title).toBe('New task');
    req.flush(makeTask(10, 'todo', 'New task'));
    fixture.detectChanges();

    expect(cmp.tasks().length).toBe(1);
    expect(cmp.showForm()).toBe(false);
    expect(cmp.draft.title).toBe('');
    expect(el().querySelectorAll('.task-card').length).toBe(1);
  });

  it('ignores a blank title', () => {
    http.expectOne('/api/v1/tasks').flush([]);
    fixture.componentInstance.draft.title = '   ';
    fixture.componentInstance.createTask();
    http.expectNone('/api/v1/tasks');
  });

  it('updates the status of a task', () => {
    http.expectOne('/api/v1/tasks').flush([makeTask(1, 'todo')]);
    const cmp = fixture.componentInstance;
    cmp.changeStatus(cmp.tasks()[0], 'in_progress');
    http.expectOne('/api/v1/tasks/1').flush(makeTask(1, 'in_progress'));
    expect(cmp.tasks()[0].status).toBe('in_progress');
    expect(cmp.busy()).toBe(false);
  });

  it('deletes a task only after confirmation', () => {
    http.expectOne('/api/v1/tasks').flush([makeTask(1, 'todo')]);
    const cmp = fixture.componentInstance;

    vi.spyOn(window, 'confirm').mockReturnValue(false);
    cmp.remove(cmp.tasks()[0]);
    http.expectNone('/api/v1/tasks/1');
    expect(cmp.tasks().length).toBe(1);

    vi.spyOn(window, 'confirm').mockReturnValue(true);
    cmp.remove(cmp.tasks()[0]);
    http.expectOne('/api/v1/tasks/1').flush(null);
    expect(cmp.tasks().length).toBe(0);
  });

  it('surfaces a delete failure', () => {
    http.expectOne('/api/v1/tasks').flush([makeTask(1, 'todo')]);
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    const cmp = fixture.componentInstance;
    cmp.remove(cmp.tasks()[0]);
    http.expectOne('/api/v1/tasks/1').flush('nope', { status: 500, statusText: 'Server Error' });
    expect(cmp.error()).toContain('could not be deleted');
    expect(cmp.tasks().length).toBe(1);
    expect(cmp.busy()).toBe(false);
  });
});
