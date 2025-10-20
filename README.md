## Getting Started

### Prerequisites

- Go 1.21 or higher


### Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd hng1
```

2. Install dependencies:
```bash
go mod tidy
```

5. Run the application:
```bash
go run main.go
```

The server will start on `http://localhost:8000`

## API Endpoints

# Create a string
`POST` - http://localhost:8000/strings 
  '{"value": "ekondo"}'

# Get specific string
`GET` - http://localhost:8000/strings/ekondo

# Get all palindromes
`GET` - http://localhost:8000/strings?is_palindrome=true

# Natural language query
`GET` - http://localhost:8000/strings/filter-by-natural-language?query=all%20single%20word%20palindromic%20strings

# Delete string
`DELETE` - http://localhost:8000/strings/ekondo
