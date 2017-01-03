package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/mail"
	"net/smtp"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Config file for the mail
type Config struct {
	MailSender   string   `json:"mail_sender"`
	MailServer   string   `json:"mail_server"`
	MailUsername string   `json:"mail_username"`
	MailPassword string   `json:"mail_password"`
	FtpDir       string   `json:"ftp_dir"`
	OfficeDir    string   `json:"office_dir"`
	DispenseDir  string   `json:"dispense_dir"`
	Recipients   []string `json:"recipients"`
}

type loginAuth struct {
	username, password string
}

// LoginAuth Usage:
// auth := LoginAuth("loginname", "password")
// err := smtp.SendMail(smtpServer + ":25", auth, fromAddress, toAddresses, []byte(message))
// or
// client, err := smtp.Dial(smtpServer)
// client.Auth(LoginAuth("loginname", "password"))
func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte(a.username), nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch string(fromServer) {
		case "Username:":
			return []byte(a.username), nil
		case "Password:":
			return []byte(a.password), nil
		default:
			return nil, errors.New("Unkown fromServer")
		}
	}
	return nil, nil
}

func getLatestVersion(ftpDir string) []string {
	files, err := ioutil.ReadDir(ftpDir)
	if err != nil {
		fmt.Println("Cannot find ftp directory!")
		os.Exit(1)
	}
	var dispensePool []string
	var officePool []string
	var schedulerPool []string
	var registerPool []string

	for _, v := range files {
		if strings.HasPrefix(v.Name(), "Dispense") {
			dispensePool = append(dispensePool, v.Name())
		} else if strings.HasPrefix(v.Name(), "Office") {
			officePool = append(officePool, v.Name())
		} else if strings.HasPrefix(v.Name(), "Register") {
			registerPool = append(registerPool, v.Name())
		} else if strings.HasPrefix(v.Name(), "Scheduler") {
			schedulerPool = append(schedulerPool, v.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dispensePool)))
	sort.Sort(sort.Reverse(sort.StringSlice(officePool)))
	sort.Sort(sort.Reverse(sort.StringSlice(registerPool)))
	sort.Sort(sort.Reverse(sort.StringSlice(schedulerPool)))

	var versions []string
	if len(dispensePool) > 0 {
		//fmt.Println(dispensePool[0])
		versions = append(versions, strings.Trim(strings.Trim(dispensePool[0], "Dispense"), ".zip"))
	}
	if len(officePool) > 0 {
		//fmt.Println(officePool[0])
		versions = append(versions, strings.Trim(strings.Trim(officePool[0], "Office"), ".zip"))
	}
	if len(registerPool) > 0 {
		//fmt.Println(registerPool[0])
		versions = append(versions, strings.Trim(strings.Trim(registerPool[0], "Register"), ".zip"))
	}
	if len(schedulerPool) > 0 {
		//fmt.Println(schedulerPool[0])
		versions = append(versions, strings.Trim(strings.Trim(schedulerPool[0], "Scheduler"), ".zip"))
	}

	return versions
}

func getLatestBuildLog(directory string) string {
	// Get office log
	out, err := exec.Command("svn", "log", directory, "-l", "30", "--search", "zbuild").Output()
	re := regexp.MustCompile("r([0-9]+)")

	//fmt.Println(re.FindAllString(string(out), 2))
	revisions := re.FindAllString(string(out), 2)

	revisionFrom, err := strconv.Atoi(revisions[1][1:])
	revisionTo, err := strconv.Atoi(revisions[0][1:])
	if err != nil {
		fmt.Println("Cannot find the correct revision for: " + directory + ", Atoi failed")
		os.Exit(1)
	}
	revisionFrom++
	revisionTo--

	officeRange := "-r" + strconv.Itoa(revisionFrom) + ":" + strconv.Itoa(revisionTo)
	//fmt.Println(officeRange)
	out, err = exec.Command("svn", "log", directory, officeRange).Output()
	buildLog := string(out)

	// Remove commit info lines (author, no of lines, etc)
	reg := regexp.MustCompile(`r\d+ .+\s+\n`)
	buildLog = reg.ReplaceAllString(buildLog, "")
	buildLog = regexp.MustCompile(`-+\s+\n`).ReplaceAllString(buildLog, "")

	// Split
	var lines []string
	logLines := strings.Split(buildLog, "\n")
	for _, v := range logLines {
		if !regexp.MustCompile(`^- .*`).Match([]byte(v)) && v != "" {
			v = "- " + v
		}
		lines = append(lines, v)
	}

	return strings.Join(lines, "<br>")
}

func main() {
	// Read Config
	var conf Config
	raw, err := ioutil.ReadFile("./config.json")
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	json.Unmarshal(raw, &conf)

	auth := LoginAuth(conf.MailUsername, conf.MailPassword)

	officeLog := getLatestBuildLog(conf.OfficeDir)
	dispenseLog := getLatestBuildLog(conf.DispenseDir)
	versions := getLatestVersion(conf.FtpDir)

	greet := "Hi All, <br>The latest software update has been pushed to "
	if len(os.Args) > 1 {
		for _, v := range os.Args[1:] {
			greet += v + ", "
		}
	} else {
		greet += "test "
	}
	greet += "streams."

	fmt.Println("Processing Mail...")

	from := mail.Address{Name: "Build Bot", Address: conf.MailSender}
	var to string
	for _, v := range conf.Recipients {
		to += (&mail.Address{Name: "", Address: v}).String() + ", "
	}
	subj := "Software Update Release " + time.Now().Local().Format("01/02/2006")
	body := "<span style=\"font-family: Calibri, sans-serif; font-size: 15;\">"
	body += "\n" + greet
	body += "\n\n<br><br><u>The latest software versions are:</u>"
	body += "\n<br><strong>Dispense: </strong>" + "<span style=\"color:#2ED03C\">" + versions[0] + "</span>"
	body += "\n<br><strong>Office: </strong>" + "<span style=\"color:#2ED03C\">" + versions[1] + "</span>"
	body += "\n<br><strong>Register: </strong>" + "<span style=\"color:#2ED03C\">" + versions[2] + "</span>"
	body += "\n<br><strong>Scheduler: </strong>" + "<span style=\"color:#2ED03C\">" + versions[3] + "</span>"
	body += "\n<br><br><u>Dispense Changelog: </u>"
	body += "\n<br>" + dispenseLog
	body += "\n<br><br><u>Office Changelog: </u>"
	body += "\n<br>" + officeLog
	body += "\n\n<br><br>Thanks, <br>build-bot</span><br><br>P.S. Dont reply to me I'm a bot."

	// Setup headers
	headers := make(map[string]string)
	headers["From"] = from.String()
	headers["To"] = to
	headers["Date"] = time.Now().Local().Format("Mon, 2 Jan 2006 15:04:05 -0700 (MST)")
	headers["Subject"] = subj
	headers["Content-Type"] = "text/html"

	// Setup message
	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	fmt.Println(message)

	// Send Message
	client, err := smtp.Dial(conf.MailServer + ":25")
	defer client.Quit()

	err = client.Auth(auth)
	client.Mail(conf.MailSender)

	for _, rc := range conf.Recipients {
		client.Rcpt(rc)
	}

	w, err := client.Data()
	defer w.Close()

	if err != nil {
		log.Fatal(err)
	}
	w.Write([]byte(message))

	log.Println("Success")

	if err != nil {
		log.Fatal(err)
	}
}
