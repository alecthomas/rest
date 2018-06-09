# An opinionated API for building RESTful services


```go
type todoService struct {}
func (t *todoService) CreateTodo(todo *Todo) error { return nil }
func (t *todoService) DeleteTodo(id string) (*Todo, error) { return nil, nil }
func (t *todoService) AddChild(parent string, todo *Todo) error { return nil }

todo := &todoService{}

api := rest.New()
api.Post("/todo", todo.CreateTodo).
api.Delete("/todo/:id", todo.DeleteTodo).
api.Post("/todo/:id/children", todo.AddChild).
```
