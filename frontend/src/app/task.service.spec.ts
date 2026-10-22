import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { Task, TaskInput } from './task.model';
import { TaskService } from './task.service';

const sample: Task = {
  id: 7,
  title: 'Prepare release',
  description: '',
  status: 'todo',
  created_at: '2026-01-01T12:00:00Z',
  updated_at: '2026-01-01T12:00:00Z'
};

describe('TaskService', () => {
  let service: TaskService;
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    service = TestBed.inject(TaskService);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  it('lists tasks', () => {
    let result: Task[] = [];
    service.list().subscribe(tasks => (result = tasks));
    const req = http.expectOne('/api/v1/tasks');
    expect(req.request.method).toBe('GET');
    req.flush([sample]);
    expect(result).toEqual([sample]);
  });

  it('creates a task', () => {
    const input: TaskInput = { title: 'Prepare release', description: '', status: 'todo' };
    service.create(input).subscribe();
    const req = http.expectOne('/api/v1/tasks');
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual(input);
    req.flush(sample);
  });

  it('updates only the status', () => {
    service.updateStatus(7, 'done').subscribe();
    const req = http.expectOne('/api/v1/tasks/7');
    expect(req.request.method).toBe('PATCH');
    expect(req.request.body).toEqual({ status: 'done' });
    req.flush({ ...sample, status: 'done' });
  });

  it('deletes a task', () => {
    service.delete(7).subscribe();
    const req = http.expectOne('/api/v1/tasks/7');
    expect(req.request.method).toBe('DELETE');
    req.flush(null);
  });
});
