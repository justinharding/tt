package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	dateTimeFormat = "2006-01-02 15:04:05"
	dateFormat     = "2006-01-02"
	endOfDatePos   = 21
)

var timeLogFile string

// Entry represents a parsed log entry
type Entry struct {
	Project  string
	Segments []string
	Hours    float64
}

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}

	var (
		group  bool
		file   string
		action string
		args   []string
	)

	// Parse flags and arguments, allowing flags anywhere
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "-g", "-group":
			group = true
		case "-file":
			if i+1 < len(os.Args) {
				file = os.Args[i+1]
				i++
			}
		default:
			if strings.HasPrefix(arg, "-") {
				// Unknown flag, skip
				continue
			}
			if action == "" {
				action = arg
			} else {
				args = append(args, arg)
			}
		}
	}

	if f := os.Getenv("TIMELOG"); f != "" {
		timeLogFile = f
	}
	commands := []string{"in", "out", "sw", "switch", "cur", "st", "last", "hours", "td", "hoursago", "yd", "thisweek", "tw", "validate", "timelog"}
	// If file not set, check if last arg is a filename (not an action or flag)
	if file == "" && len(args) > 0 &&
		!strings.HasPrefix(args[len(args)-1], "-") &&
		!isInteger(args[len(args)-1]) &&
		!slices.Contains(commands, args[len(args)-1]) &&
		!strings.HasPrefix(args[len(args)-1], "last") &&
		!strings.HasPrefix(args[len(args)-1], "yd") &&
		!strings.HasPrefix(args[len(args)-1], "lw") &&
		!strings.HasPrefix(args[len(args)-1], "ins") &&
		!strings.HasPrefix(args[len(args)-1], "cat") {
		file = args[len(args)-1]
		args = args[:len(args)-1]
	}

	if file != "" {
		timeLogFile = file
	}

	// Dispatch tagged actions
	switch {
	case strings.HasPrefix(action, "last") && strings.TrimLeft(action[len("last"):], "^") == "":
		handleLast(action, args)
		return
	case strings.HasPrefix(action, "yd") && strings.TrimLeft(action[len("yd"):], "^") == "":
		handleYd(action, args, group)
		return
	case strings.HasPrefix(action, "lw") && strings.TrimLeft(action[len("lw"):], "^") == "":
		handleLw(action, args, group)
		return
	case strings.HasPrefix(action, "ins") && strings.TrimLeft(action[len("ins"):], "^") == "":
		handleIns(action, args)
		return
	case strings.HasPrefix(action, "cat") && strings.TrimLeft(action[len("cat"):], "^") == "":
		handleCat(action,
			args)
		return
	}

	// Dispatch regular actions
	switch action {
	case "in", "sw", "switch":
		lastType, err := lastEntryType()
		if err != nil {
			fmt.Println("Error reading last entry:", err)
			os.Exit(1)
		}
		if action == "in" && lastType != "o" {
			fmt.Println("Cannot clock in: last entry is not an 'o' (out) entry.")
			os.Exit(1)
		}
		if (action == "sw" || action == "switch") && lastType != "i" {
			fmt.Println("Cannot switch: last entry is not an 'i' (in) entry.")
			os.Exit(1)
		}

		var project string
		if len(args) > 0 {
			project = strings.Join(args, " ")
		} else {
			projects, err := lastNProjects(10)
			if err != nil || len(projects) == 0 {
				fmt.Println("No previous projects found.")
				os.Exit(1)
			}
			fmt.Println("Select a project:")
			for i, p := range projects {
				fmt.Printf("%d: %s\n", i+1, p)
			}
			fmt.Print("Enter number: ")
			var choice int
			_, err = fmt.Scanf("%d", &choice)
			if err != nil || choice < 1 || choice > len(projects) {
				fmt.Println("Invalid selection.")
				os.Exit(1)
			}
			project = projects[choice-1]
		}
		if action == "in" {
			if err := clockIn(project); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
		} else {
			if err := switchProject(project); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
		}
	case "out":
		lastType, err := lastEntryType()
		if err != nil {
			fmt.Println("Error reading last entry:", err)
			os.Exit(1)
		}
		if lastType != "i" {
			fmt.Println("Cannot out: last entry is not an 'i' (in) entry.")
			os.Exit(1)
		}

		project := strings.Join(args, " ")
		if err := clockOut(project); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
	case "cur", "st":
		if proj, err := currentProject(); err == nil {
			fmt.Println("Current project:", proj)
		} else {
			fmt.Println("Error:", err)
		}
	case "last":
		count := 1
		if len(args) > 0 {
			fmt.Sscanf(args[0], "%d", &count)
		}
		if proj, err := lastProjectN(count); err == nil {
			fmt.Println("Last closed project:", proj)
		} else {
			fmt.Println("Error:", err)
		}
	case "hours", "td":
		hours, _, _, entries, err := hoursToday(group)
		if err != nil {
			fmt.Println("Error:", err)
		} else if group {
			DisplayHierTotals(entries)
		} else {
			fmt.Printf("Hours worked today: %.2f\n", hours)
		}
	case "hoursago", "yd":
		var days int
		if len(args) > 0 {
			fmt.Sscanf(args[0], "%d", &days)
		}
		hours, _, _, entries, err := hoursForDay(days, group)
		if err != nil {
			fmt.Println("Error:", err)
		} else if group {
			DisplayHierTotals(entries)
		} else {
			fmt.Printf("Hours worked %d days ago: %.2f\n", days, hours)
		}
	case "thisweek", "tw":
		hours, _, _, entries, err := hoursThisWeek(group)
		if err != nil {
			fmt.Println("Error:", err)
		} else if group {
			DisplayHierTotals(entries)
		} else {
			fmt.Printf("Hours worked this week: %.2f\n", hours)
		}
	case "timelog":
		fmt.Println(getTimelogFile())
		os.Exit(1)
	case "validate":
		if err := validateTimelogFile(getTimelogFile()); err != nil {
			fmt.Println("Validation error:", err)
			os.Exit(1)
		}
		return
	default:
		usage()
	}
}

func usage() {
	fmt.Println(`Usage: timelog <action> [project] [options] [filename]
Actions:
  in <project>      - clock into project (only if last entry is 'o')
  out <project>     - clock out of project (only if last entry is 'i')
  sw <project>      - switch projects (only if last entry is 'i')
  cur               - show currently open project
  last              - show last closed project
  hours/td          - show hours worked today
  hoursago/yd       - show hours worked N days ago
  thisweek          - show hours worked this week
  yd                - show hours for yesterday
  lw                - show hours for last week
  validate          - validate timelog file for out-of-order entries
Options:
  -group            - group output by project
  -file <filename>  - specify timelog file
  [filename]        - specify timelog file as last argument

	last, yd, lw, cat can all take a param N to indicate how many days back, e.g. "yd 3" for 3 days ago.
	they can also be suffixed with ^ characters, e.g. "yd^^" for 2 days ago.`)

	fmt.Println("If no -file option is given, the TIMELOG environment variable is used if set, otherwise 'timelog.txt' in the current directory.")
}

func getTimelogFile() string {
	if timeLogFile != "" {
		return timeLogFile
	}

	return "timelog.txt"
}

func handleLast(action string, args []string) {
	count := 1 + strings.Count(action[len("last"):], "^")
	if len(args) > 0 {
		fmt.Sscanf(args[0], "%d", &count)
	}
	if proj, err := lastProjectN(count); err == nil {
		fmt.Println("Last closed project:", proj)
	} else {
		fmt.Println("Error:", err)
	}
}

func handleYd(action string, args []string, group bool) {
	count := 1 + strings.Count(action[len("yd"):], "^")
	hours, _, _, entries, err := hoursForDay(count, group)
	if err != nil {
		fmt.Println("Error:", err)
	} else if group {
		DisplayHierTotals(entries)
	} else {
		fmt.Printf("Hours worked %d days ago: %.2f\n", count, hours)
	}
}

func handleLw(action string, args []string, group bool) {
	count := strings.Count(action[len("lw"):], "^")
	if len(args) > 0 {
		fmt.Sscanf(args[0], "%d", &count)
	}
	hours, _, _, entries, err := hoursForWeek(count, group)
	if err != nil {
		fmt.Println("Error:", err)
	} else if group {
		DisplayHierTotals(entries)
	} else {
		switch count {
		case 0:
			fmt.Printf("Hours worked this week: %.2f\n", hours)
		case 1:
			fmt.Printf("Hours worked last week: %.2f\n", hours)
		default:
			fmt.Printf("Hours worked %d weeks ago: %.2f\n", count, hours)
		}
	}
}

func handleIns(action string, args []string) {
	count := 1 + strings.Count(action[len("cat"):], "^")
	if len(args) > 0 {
		fmt.Sscanf(args[0], "%d", &count)
	}
	err := CatInEntries(count)
	if err != nil {
		fmt.Println("Error:", err)
	}
}

func handleCat(action string, args []string) {
	count := 1 + strings.Count(action[len("cat"):], "^")
	if len(args) > 0 {
		fmt.Sscanf(args[0], "%d", &count)
	}
	err := CatAllEntries(count)
	if err != nil {
		fmt.Println("Error:", err)
	}
}

func validateTimelogFile(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	var lastTime time.Time
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.HasPrefix(line, "i ") || strings.HasPrefix(line, "o ") {
			parts := strings.Fields(line)
			if len(parts) < 3 {
				fmt.Printf("Warning: line %d malformed: %s\n", lineNum, line)
				continue
			}
			datetime := parts[1] + " " + parts[2]
			t, err := time.ParseInLocation(dateTimeFormat, datetime, time.Local)
			if err != nil {
				fmt.Printf("Warning: line %d invalid time: %s\n", lineNum, line)
				continue
			}
			if !lastTime.IsZero() && t.Before(lastTime) {
				fmt.Printf("Warning: line %d time %s before previous entry (%s)\n", lineNum, t.Format(dateTimeFormat), lastTime.Format(dateTimeFormat))
			}
			lastTime = t
		}
	}
	return nil
}

func clockIn(project string) error {
	if alreadyCheckedIn() {
		return errors.New("already checked in")
	}
	entry := fmt.Sprintf("i %s %s\n", time.Now().Format(dateTimeFormat), project)
	return appendToFile(entry)
}

func clockOut(project string) error {
	if alreadyCheckedOut() {
		return errors.New("already checked out")
	}
	entry := fmt.Sprintf("o %s %s\n", time.Now().Format(dateTimeFormat), project)
	return appendToFile(entry)
}

func switchProject(project string) error {
	if alreadyCheckedOut() {
		return errors.New("not checked in")
	}
	if current, _ := currentProject(); current == project {
		return errors.New("already checked in to this project")
	}
	if err := clockOut(""); err != nil {
		return err
	}
	return clockIn(project)
}

func appendToFile(entry string) error {
	f, err := os.OpenFile(getTimelogFile(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(entry)
	return err
}

func alreadyCheckedIn() bool {
	last, _ := lastEntry()
	return strings.HasPrefix(last, "i")
}

func alreadyCheckedOut() bool {
	last, _ := lastEntry()
	return strings.HasPrefix(last, "o")
}

func lastEntry() (string, error) {
	f, err := os.Open(getTimelogFile())
	if err != nil {
		return "", err
	}
	defer f.Close()
	var last string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		last = scanner.Text()
	}
	return last, scanner.Err()
}

func currentProject() (string, error) {
	f, err := os.Open(getTimelogFile())
	if err != nil {
		return "", err
	}
	defer f.Close()
	var lastIn string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "i ") {
			lastIn = strings.TrimSpace(line[endOfDatePos:])
		}
	}
	if lastIn == "" {
		return "", errors.New("no current project")
	}
	return lastIn, nil
}

func lastProjectN(count int) (string, error) {
	f, err := os.Open(getTimelogFile())
	if err != nil {
		return "", err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Find all "o" entry indices (closed projects)
	var outIndices []int
	for i, line := range lines {
		if strings.HasPrefix(line, "o ") {
			outIndices = append(outIndices, i)
		}
	}

	if len(outIndices) < count || count < 1 {
		return "", errors.New("not enough closed projects")
	}

	// Get the nth last "o" entry
	outIdx := outIndices[len(outIndices)-count]

	// Find the most recent "i" entry before this "o"
	var lastIn string
	for i := outIdx - 1; i >= 0; i-- {
		if strings.HasPrefix(lines[i], "i ") {
			lastIn = strings.TrimSpace(lines[i][endOfDatePos:])
			break
		}
	}

	if lastIn == "" {
		return "", errors.New("no closed project found for given count")
	}
	return lastIn, nil
}

func lastNProjects(n int) ([]string, error) {
	f, err := os.Open(getTimelogFile())
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var allProjects []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "i ") {
			proj := strings.TrimSpace(line[endOfDatePos:])
			if proj != "" {
				allProjects = append(allProjects, proj)
			}
		}
	}
	// Reverse for most recent first
	for i, j := 0, len(allProjects)-1; i < j; i, j = i+1, j-1 {
		allProjects[i], allProjects[j] = allProjects[j], allProjects[i]
	}
	// Deduplicate, preserving order
	unique := []string{}
	seen := make(map[string]bool)
	for _, proj := range allProjects {
		if !seen[proj] {
			unique = append(unique, proj)
			seen[proj] = true
			if len(unique) == n {
				break
			}
		}
	}
	return unique, nil
}

// Returns hours worked for today
func hoursToday(group bool) (float64, map[string]float64, map[string]map[string]float64, []string, error) {
	return hoursForDay(0, group)
}

func hoursForRange(startDate, endDate string, group bool) (float64, map[string]float64, map[string]map[string]float64, []string, error) {
	f, err := os.Open(getTimelogFile())
	if err != nil {
		return 0, nil, nil, nil, err
	}
	defer f.Close()

	var inTimes, outTimes []time.Time
	var inProjects []string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		date := parts[1]
		datetime := parts[1] + " " + parts[2]
		if date < startDate || date > endDate {
			continue
		}
		if strings.HasPrefix(line, "i ") {
			t, err := time.ParseInLocation(dateTimeFormat, datetime, time.Local)
			if err == nil {
				inTimes = append(inTimes, t)
				project := strings.Join(parts[3:], " ")
				inProjects = append(inProjects, project)
			}
		}
		if strings.HasPrefix(line, "o ") {
			t, err := time.ParseInLocation(dateTimeFormat, datetime, time.Local)
			if err == nil {
				outTimes = append(outTimes, t)
			}
		}
	}

	// Only append time.Now() if there is one more in than out
	lastType, _ := lastEntryType()
	today := time.Now().Format(dateFormat)
	if lastType == "i" && today >= startDate && today <= endDate && len(inTimes) == len(outTimes)+1 {
		outTimes = append(outTimes, time.Now())
	}

	var entries []string
	var total float64
	// Only pair up to the minimum of inTimes and outTimes
	n := min(len(outTimes), len(inTimes))
	for i := range n {
		dur := outTimes[i].Sub(inTimes[i])
		if dur > 0 {
			total += dur.Hours()
			entries = append(entries, fmt.Sprintf("%f %s", dur.Hours(), inProjects[i]))
		}
	}

	if group {
		projectTotals, projectPaths := groupFlatTotals(entries)
		return total, projectTotals, projectPaths, entries, nil
	}
	return total, nil, nil, entries, nil
}

func hoursForDay(daysAgo int, group bool) (float64, map[string]float64, map[string]map[string]float64, []string, error) {
	targetDate := time.Now().AddDate(0, 0, -daysAgo).Format(dateFormat)
	return hoursForRange(targetDate, targetDate, group)
}

func hoursThisWeek(group bool) (float64, map[string]float64, map[string]map[string]float64, []string, error) {
	now := time.Now()
	offset := int(now.Weekday())
	if offset == 0 {
		offset = 6
	} else {
		offset = offset - 1
	}
	monday := now.AddDate(0, 0, -offset)
	sunday := monday.AddDate(0, 0, 6)
	return hoursForRange(monday.Format(dateFormat), sunday.Format(dateFormat), group)
}

func hoursForWeek(weeksAgo int, group bool) (float64, map[string]float64, map[string]map[string]float64, []string, error) {
	now := time.Now()
	offset := int(now.Weekday())
	if offset == 0 {
		offset = 6
	} else {
		offset = offset - 1
	}
	monday := now.AddDate(0, 0, -offset-7*weeksAgo)
	sunday := monday.AddDate(0, 0, 6)
	return hoursForRange(monday.Format(dateFormat), sunday.Format(dateFormat), group)
}

func groupFlatTotals(entries []string) (map[string]float64, map[string]map[string]float64) {
	projectTotals := make(map[string]float64)
	projectPaths := make(map[string]map[string]float64)
	for _, entry := range entries {
		parts := strings.Fields(entry)
		if len(parts) < 2 {
			continue
		}
		project := strings.Split(parts[1], ":")[0]
		path := parts[1]
		duration, err := time.ParseDuration(parts[0] + "h")
		if err != nil {
			continue
		}
		hours := duration.Hours()
		projectTotals[project] += hours
		if projectPaths[project] == nil {
			projectPaths[project] = make(map[string]float64)
		}
		projectPaths[project][path] += hours
	}
	return projectTotals, projectPaths
}

// Parse entries into structured data
func parseEntries(entries []string) []Entry {
	var result []Entry
	for _, entry := range entries {
		parts := strings.Fields(entry)
		if len(parts) < 2 {
			continue
		}
		segments := strings.Split(parts[1], ":")
		duration, err := time.ParseDuration(parts[0] + "h")
		if err != nil {
			continue
		}
		result = append(result, Entry{
			Project:  segments[0],
			Segments: segments,
			Hours:    duration.Hours(),
		})
	}
	return result
}

// Group and display hierarchically
func DisplayHierTotals(entries []string) {
	parsed := parseEntries(entries)
	projectTotals := make(map[string]float64)
	subTotals := make(map[string]map[string]float64)
	subSubTotals := make(map[string]map[string]float64)

	for _, e := range parsed {
		projectTotals[e.Project] += e.Hours
		if len(e.Segments) > 1 {
			sub := e.Segments[1]
			if subTotals[e.Project] == nil {
				subTotals[e.Project] = make(map[string]float64)
			}
			subTotals[e.Project][sub] += e.Hours
			if len(e.Segments) > 2 {
				path := strings.Join(e.Segments[2:], ":")
				if subSubTotals[sub] == nil {
					subSubTotals[sub] = make(map[string]float64)
				}
				subSubTotals[sub][path] += e.Hours
			}
		}
	}

	for project, total := range projectTotals {
		fmt.Printf("%15.2fh  %s\n", total, project)
		for sub, subTotal := range subTotals[project] {
			fmt.Printf("%15.2fh    %s\n", subTotal, sub)
			for path, pathTotal := range subSubTotals[sub] {
				fmt.Printf("%15.2fh      %s\n", pathTotal, path)
			}
		}
	}
	fmt.Println("--------------------")
	fmt.Printf("%15.2fh\n", sumMap(projectTotals))
}

func sumMap(m map[string]float64) float64 {
	var total float64
	for _, v := range m {
		total += v
	}
	return total
}

func CatInEntries(days int) error {
	file, err := os.Open(getTimelogFile())
	if err != nil {
		return err
	}
	defer file.Close()

	var entries []string
	daySet := make(map[string]struct{})
	var daysList []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 3 || parts[0] != "i" {
			continue
		}
		date := parts[1][:10]
		entries = append(entries, line)
		if _, exists := daySet[date]; !exists {
			daysList = append(daysList, date)
			daySet[date] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	if days > len(daysList) {
		days = len(daysList)
	}
	lastDays := daysList[len(daysList)-days:]

	for _, entry := range entries {
		parts := strings.Fields(entry)
		date := parts[1][:10]
		if slices.Contains(lastDays, date) {
			fmt.Println(entry)
		}
	}
	return nil
}

func CatAllEntries(days int) error {
	file, err := os.Open(getTimelogFile())
	if err != nil {
		return err
	}
	defer file.Close()

	var entries []string
	daySet := make(map[string]struct{})
	var daysList []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		date := parts[1][:10]
		entries = append(entries, line)
		if _, exists := daySet[date]; !exists {
			daysList = append(daysList, date)
			daySet[date] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	if days > len(daysList) {
		days = len(daysList)
	}
	lastDays := daysList[len(daysList)-days:]

	for _, entry := range entries {
		parts := strings.Fields(entry)
		date := parts[1][:10]
		if slices.Contains(lastDays, date) {
			fmt.Println(entry)
		}
	}
	return nil
}

func lastEntryType() (string, error) {
	f, err := os.Open(getTimelogFile())
	if err != nil {
		return "", err
	}
	defer f.Close()
	var lastType string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "i ") {
			lastType = "i"
		} else if strings.HasPrefix(line, "o ") {
			lastType = "o"
		}
	}
	return lastType, nil
}

func isInteger(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}
