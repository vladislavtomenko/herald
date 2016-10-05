package main
import (
  "os"
  "os/exec"
  "time"
  "fmt"
  "flag"
  "log"
  "strconv"
  "net/http"
  "io/ioutil"
  "encoding/json"
  "io"
  "github.com/gorilla/handlers"
  "github.com/foomo/htpasswd"
  "github.com/abbot/go-http-auth"
)

type Configuration struct {
  Listen   string
  Port  int
  AsteriskCallDestintationDir string
  CallFilesSourceDir  string
  AcknowledgeTimeShift  int64
  AsteriskWavDir  string
  WavFileName  string
  VoiceSynthesizerBinPath string
  AuthFile string
  Enabled  bool
  IntroMessage string
  OutroMessage string
}


func Secret(user, realm string) string {
  passwords, err := htpasswd.ParseHtpasswdFile(configuration.AuthFile)
  if err != nil {
    return ""
  }
  for User, Pass := range passwords {
    if User == user {
      return Pass
    }
  }
  return ""
}

func FprintHTMLError(w http.ResponseWriter, err error) {
  fmt.Fprintf(w, "{\n\"status\": \"error\",\n\"message\": \"%s\"\n}", err)
}

func FprintHTMLInfo(w http.ResponseWriter, message string) {
  fmt.Fprintf(w, "{\n\"status\": \"ok\",\n\"message\": \"%s\"\n}", message)
}

func FprintHTMLDisabled(w http.ResponseWriter) {
  message := "Service is disable by acknowledgement. Will been enabled again at " + time.Unix(AckDate, 0).Format(time.RFC3339)
  if !configuration.Enabled {
    message = "Service is disabled in a configuration file"
  }
  fmt.Fprintf(w, "{\n\"status\": \"disabled\",\n\"message\": \"%s\"\n}", message)
}

func CopyFiles (SourceDir string, DestinationDir string) (err error) {

    files, err := ioutil.ReadDir(SourceDir)
  if err != nil {
    return err
  }

  for _, file := range files {
    // Open original file
      originalFile, err := os.Open(SourceDir+file.Name())
      if err != nil {
          return err
      }
      defer originalFile.Close()

      // Create new file
      newFile, err := os.Create(DestinationDir+file.Name())
      if err != nil {
          return err
      }
      defer newFile.Close()

      // Copy the bytes to destination from source
      _, err = io.Copy(newFile, originalFile)
      if err != nil {
          return err
      }    
      // Commit the file contents
      // Flushes memory to disk
      err = newFile.Sync()
      if err != nil {
        return err
      }
  }
  return
}

func GenerateMessage (r *auth.AuthenticatedRequest) (string) {
  r.ParseForm()
  HostName := r.Form.Get("host")
  ServiceName := r.Form.Get("service")
  ServiceState := r.Form.Get("state")
  AlertMessage := r.Form.Get("message")

  message := configuration.IntroMessage + " " + ServiceName + " status is " + ServiceState + " on " + HostName + " " + AlertMessage + " " + configuration.OutroMessage
  log.Print(message)
  return message 
}

func CheckIfServiceEnabled() (bool) {
  if !configuration.Enabled {
    return false
  } else {
    if time.Now().Unix() >= AckDate {
     return true
   }
  }
  return false
}

func AckHandler (w http.ResponseWriter, r *auth.AuthenticatedRequest) {
  r.ParseForm()
  duration, err := strconv.ParseInt(r.Form.Get("duration"), 10, 64)
  log.Println("duration: " + strconv.FormatInt(duration, 10))
  if err != nil {
    duration = configuration.AcknowledgeTimeShift
  }
  AckDate = time.Now().Unix() + duration
  FprintHTMLInfo(w, "Service has been disabled until " + time.Unix(AckDate, 0).Format(time.RFC3339))
}

func EnableHandler (w http.ResponseWriter, r *auth.AuthenticatedRequest) {
  if !configuration.Enabled {
    FprintHTMLInfo(w, "Impossible to enable service, disbled in a configuration file")
    return
  }
  AckDate = time.Now().Unix()
  FprintHTMLInfo(w, "Service has been enabled")
}

func StatusHandler (w http.ResponseWriter, r *auth.AuthenticatedRequest) {
  if !CheckIfServiceEnabled() {
    FprintHTMLDisabled(w)
    return
  }
  FprintHTMLInfo(w, "Service is enabled")
}

func NotFoundHandler   (w http.ResponseWriter, r *auth.AuthenticatedRequest) {
  fmt.Fprintf(w, "{\n\"error\": \"error\",\n\"message\": \"page not found\"\n}")
}

func CallHandler(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
  if !CheckIfServiceEnabled() {
    FprintHTMLDisabled(w)
    return
  }

  var SourceDir = configuration.CallFilesSourceDir
  var DestinationDir = configuration.AsteriskCallDestintationDir

  //generate audo file and copy it to an asterisk dir
  var err = GenerateAudioFile(GenerateMessage(r), configuration.AsteriskWavDir, configuration.WavFileName, configuration.VoiceSynthesizerBinPath)
  if err != nil {
    FprintHTMLError(w, err)
    return
  }

  //copy call files to an asterisk call files folder
  err = CopyFiles(SourceDir, DestinationDir)
  if err != nil {
    FprintHTMLError(w, err)
    return
  }

  FprintHTMLInfo(w, "Successfully copied call files")
}

func GenerateAudioFile(Message string, AsteriskWavDir string, WavFileName string, VoiceSynthesizerBinPath string) (err error) {
  cmd :=  "echo \""+Message+"\" | " + VoiceSynthesizerBinPath + " " + AsteriskWavDir + WavFileName
  log.Println(cmd)
  out, err := exec.Command("/bin/sh", "-c", cmd).Output()
  log.Println(out)
  return err
}

func GetConfig(ConfigurationFilePath string) (error) {
  file, _ := os.Open(ConfigurationFilePath)
  decoder := json.NewDecoder(file)
  configuration = Configuration{}
  err := decoder.Decode(&configuration)
  return err
}

var configuration Configuration
var AckDate int64

func main() {
  var ConfigurationFilePath string

  flag.Usage = func() {
    fmt.Printf("Usage of herald:\n")
    flag.PrintDefaults()
  }

  flag.StringVar(&ConfigurationFilePath, "c", "/etc/herald/herald.conf", "specify path to a configuration file.  defaults to /etc/herald/herald.conf")
  flag.Parse()

  err := GetConfig(ConfigurationFilePath)
  if err != nil {
    fmt.Println("Configuration file error:", err)
    os.Exit(1)
  }

  var bind = configuration.Listen + ":" + strconv.Itoa(configuration.Port)
  AckDate = time.Now().Unix()

  r := http.NewServeMux()
  authenticator := auth.NewBasicAuthenticator(configuration.Listen,Secret)
    
  r.Handle("/call", authenticator.Wrap(CallHandler))
  r.Handle("/ack", authenticator.Wrap(AckHandler))
  r.Handle("/enable", authenticator.Wrap(EnableHandler))
  r.Handle("/status", authenticator.Wrap(StatusHandler))
  r.Handle("/", authenticator.Wrap(NotFoundHandler))
  http.ListenAndServe(bind, handlers.CompressHandler(handlers.LoggingHandler(os.Stdout, r)))
}
