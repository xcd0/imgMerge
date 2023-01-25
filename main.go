package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"runtime"
	"runtime/debug"
	"sync"

	_ "image/jpeg"
	_ "image/png"

	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"facette.io/natsort"
)

func main() { // {{{
	log.SetFlags(log.Lshortfile | log.Ltime | log.Lmicroseconds)
	log.SetOutput(&withGoroutineID{out: os.Stderr})

	if len(os.Args) == 0 {
		fmt.Println("引数にディレクトリのパスか、画像のパスを与えてください。")
		return
	}
	// 与えられた引数を１つづつ処理する。
	// そのうち並行処理してもいい。
	imgs := []string{""}
	for i, arg := range os.Args {
		if i == 0 {
			continue
		}
		apath, _ := filepath.Abs(arg)
		if arg == "-r" {
			bReverse = true
		} else if p, err := os.Stat(apath); os.IsNotExist(err) {
			// ファイル、ディレクトリが存在しない
			fmt.Println("引数に指定されたパスが存在しません。無視します。")
			fmt.Printf("無視する引数 : %s", arg)
		} else if p.IsDir() {
			// ディレクトリパス pdfを生成する。
			merge_img_from_dir(apath)
		} else {
			// ファイルパス pdfであればpdfから画像を生成する。
			ext := strings.ToLower(filepath.Ext(arg))
			if ext == ".jpg" || ext == ".jpeg" || ext == ".png" { // とりあえずjpgとpngのみ
				a, _ := filepath.Abs(arg)
				imgs = append(imgs, a)
			} else {
				fmt.Println("引数に指定されたファイルはjpgまたはpngではありません。無視します。")
				fmt.Printf("無視する引数 : %s", arg)
			}
		}
	}

	if len(imgs) > 1 {
		apath, _ := filepath.Abs(imgs[1])
		imgs[0] = filepath.Dir(apath)
		merge_img(imgs)
	}
} // }}}

func merge_img(imgs []string) { // {{{

	//log.Print(filepath.Dir(imgs[0]))
	n := time.Now()
	//output_path := filepath.Join(filepath.Dir(imgs[0]), filepath.Dir(imgs[0])+n.Format("2006.01.02.15.04.05"))
	output_path := ""
	fInfo, _ := os.Stat(imgs[0])
	if fInfo.IsDir() {
		//log.Print(imgs[0])
		output_path = imgs[0] + "_" + n.Format("2006.01.02.15.04.05")
	} else {
		output_path = filepath.Dir(imgs[0]) + "_" + n.Format("2006.01.02.15.04.05")
	}
	if err := os.Mkdir(output_path, 0777); err != nil {
		log.Print(err)
		r, _ := MakeRandomStr(10)
		output_path += "_" + r
		if err := os.Mkdir(output_path, 0777); err != nil {
			log.Print(err)
			log.Println("Error : 画像ファイルを保存するディレクトリが作成できませんでした。")
			log.Printf("ディレクトリ : %s での生成失敗", imgs[0])
			return
		}
	}
	log.Printf("input path  : %s", imgs[0])
	log.Printf("output path : %s", output_path)

	img_num := len(imgs) - 1

	// 画像を保持する構造体のスライスを生成
	//log.Print(imgs)
	images := make([]Image, 0, len(imgs)-1)
	for i := 1; i <= img_num; i++ {
		img, err := NewImage(imgs[i])
		if err != nil {
			// 画像が読み込めなかった
			log.Println("Error : 画像ファイルが読み込めませんでした。")
			log.Printf("画像のファイルパス : %s", imgs[i])
			log.Printf("ディレクトリ : %s での生成失敗", imgs[0])
			return
		}
		images = append(images, *img)
	}
	log.Printf("image loading finished.")

	// 並列化したい
	runtime.GOMAXPROCS(runtime.NumCPU())
	wg := new(sync.WaitGroup)

	// 2枚づつ処理する
	num := len(images) // int(math.Ceil(float64(len(images)) / 2))
	log.Printf("num : %d", num)
	// 進捗表示
	//bar := pb.Simple.Start(num)
	for i := 0; i < num; i += 2 {
		if len(images) >= i+1 {
			wg.Add(1)
			log.Printf("goroutine %d start", i)
			// 奇数枚の時の最後の画像はそのまま出力する
			// read the whole file at once
			in, err := ioutil.ReadFile(imgs[i+1])
			if err != nil {
				log.Print(err)
				return
			}

			// write the whole body at once
			out := filepath.Join(output_path, filepath.Base(imgs[i+1]))
			if err = ioutil.WriteFile(out, in, 0644); err != nil {
				log.Print(err)
				return
			}
			//bar.Increment()
			log.Printf("goroutine %d finish", i)
			wg.Done()
		} else {
			wg.Add(1)
			log.Printf("goroutine %d start", i)
			go connect(&images, &imgs, output_path, i, wg)
			log.Printf("goroutine %d finish", i)
			//bar.Increment()
		}
	}

	wg.Wait()

	//bar.Finish()

	fmt.Printf("ディレクトリ : %s での生成完了。\n", imgs[0])
	time.Sleep(time.Second * 1)
} // }}}

type withGoroutineID struct {
	out io.Writer
}

func (w *withGoroutineID) Write(p []byte) (int, error) {
	// goroutine <id> [running]:
	firstline := []byte(strings.SplitN(string(debug.Stack()), "\n", 2)[0])
	return w.out.Write(append(firstline[:len(firstline)-10], p...))
}

func connect(images *[]Image, imgs *[]string, output_path string, i int, wg *sync.WaitGroup) {
	defer wg.Done()

	// 画像を横に結合する。まず最終的な画像サイズの空白画像を生成し、その上に書き込む
	//log.Printf("input : %s", (*imgs)[i+1])
	//log.Printf("input : %s", (*imgs)[i+2])
	log.Printf("input : %s, %s", filepath.Base((*imgs)[i+1]), filepath.Base((*imgs)[i+2]))
	w := (*images)[i].width + (*images)[i+1].width
	h := int(math.Max(float64((*images)[i].height), float64((*images)[i+1].height)))
	//log.Printf("1 : (w,h) : (%d,%d) ", (*images)[i].width, (*images)[i].height)
	//log.Printf("2 : (w,h) : (%d,%d) ", (*images)[i+1].width, (*images)[i+1].height)
	//log.Printf("o : (w,h) : (%d,%d) ", w, h)
	outImg := image.NewRGBA(image.Rect(0, 0, w, h))
	rect := make([]image.Rectangle, 2)
	if bReverse { // ファイル名昇順で右から左に並べる
		rect[0] = image.Rect(0, 0, (*images)[i+1].width, (*images)[i+1].height)
		rect[1] = image.Rect((*images)[i+1].width, 0, (*images)[i+1].width+(*images)[i].width, (*images)[i].height)
	} else {
		rect[0] = image.Rect(0, 0, (*images)[i].width, (*images)[i].height)
		rect[1] = image.Rect((*images)[i].width, 0, (*images)[i].width+(*images)[i+1].width, (*images)[i+1].height)
	}
	//log.Printf("o : rect : %v", rect)
	draw.Draw(outImg, rect[0], (*images)[i].img, image.Point{0, 0}, draw.Over)
	draw.Draw(outImg, rect[1], (*images)[i+1].img, image.Point{0, 0}, draw.Over)

	base := filepath.Base((*imgs)[i+1])
	ext := strings.ToLower(filepath.Ext(base)) // ファイルパスが入っている(*imgs)と画像データが入っているimagsは1つずれている
	out := filepath.Join(output_path, base)
	OutputImage(outImg, out, ext)
	log.Printf("output : %s", out)
}

func OutputImage(outputImage image.Image, filePath string, format string) { // {{{

	dst, err := os.Create(filePath)

	if err != nil {
		log.Fatal(err)
	}

	switch format {
	case ".png":
		// PNGの場合
		err = png.Encode(dst, outputImage)
		if err != nil {
			log.Fatal(err)
		}
		break
	case ".jpeg":
		fallthrough
	case ".jpg":
		// JPGの場合
		qt := jpeg.Options{
			Quality: imgQt,
		}
		err = jpeg.Encode(dst, outputImage, &qt)
		if err != nil {
			log.Fatal(err)
		}
		break
	default:
		// 標準で対応していないフォーマットの場合
		log.Fatal("Unsupported format.")
	}
} // }}}

func MakeRandomStr(digit uint32) (string, error) { // {{{
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, digit)
	if _, err := rand.Read(b); err != nil {
		return "", errors.New("unexpected error...")
	}
	var result string
	for _, v := range b {
		result += string(letters[int(v)%len(letters)])
	}
	return result, nil
} // }}}

func merge_img_from_dir(dir string) { // {{{
	// 第一引数に画像ファイルを含むフォルダのパスをもらう
	// 直下に画像ファイルがある場合それらをまとめてpdfにする。それ以下の改装の探索はしない。
	// もし直下ではなく2階層下にのみ画像がある場合は2階層下まで探索する
	imgDirs := FindFile(dir)
	// これで画像のパスのリストが入る。ただし各リストの0番目にはフォルダ名が入っている
	for _, imgs := range imgDirs { // 画像があるディレクトリごとにリストになっている
		merge_img(imgs)
	}
} // }}}

func FindFile(root string) [][]string { // {{{
	dirEntries, err := os.ReadDir(root)
	if err != nil {
		log.Fatal(err)
	}
	// 直下に画像があるかどうかだけ先に調べる
	hasPic := false
	for _, e := range dirEntries {
		if !e.IsDir() { // ファイルのみ
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext == ".jpg" || ext == ".jpeg" || ext == ".png" { // とりあえずjpgとpngのみ
				hasPic = true
			}
		}
	}
	var out [][]string
	if hasPic { // 画像があった
		buff := make([]string, 0, 1000)
		for _, e := range dirEntries {
			if !e.IsDir() {
				if ext := strings.ToLower(filepath.Ext(e.Name())); ext == ".jpg" || ext == ".jpeg" || ext == ".png" { // とりあえずjpgとpngのみ
					if len(buff) == 0 {
						buff = append(buff, root) // 0番目にディレクトリ名を入れる
					}
					buff = append(buff, filepath.Join(root, e.Name()))
				}
			}
		}
		natsort.Sort(buff)
		out = append(out, buff)
	} else { // 画像がなかった 1階層だけ深く探索する それ以上にはいかない
		for _, e := range dirEntries {
			if e.IsDir() {
				dir := filepath.Join(root, e.Name())
				dirEntries, err := os.ReadDir(dir)
				if err != nil {
					log.Fatal(err)
				}
				buff := make([]string, 0, 1000)
				for _, e := range dirEntries { // 拡張子が画像のファイルパスを得る
					if !e.IsDir() {
						if ext := strings.ToLower(filepath.Ext(e.Name())); ext == ".jpg" || ext == ".jpeg" || ext == ".png" { // とりあえずjpgとpngのみ
							if len(buff) == 0 {
								buff = append(buff, dir) // 0番目にディレクトリ名を入れる
							}
							buff = append(buff, filepath.Join(dir, e.Name()))
						}
					}
				}
				natsort.Sort(buff)
				if len(buff) != 0 {
					out = append(out, buff)
				}
			}
		}
	}
	return out
} // }}}

type Image struct { // {{{
	img    image.Image
	width  int
	height int
}

func NewImage(path string) (*Image, error) {

	//log.Print(path)
	var i Image
	{
		file, err := os.Open(path)
		if err != nil {
			log.Print(err)
			return nil, err
		}
		defer file.Close()
		i.img, _, err = image.Decode(file)
		if err != nil {
			log.Print(err)
			return nil, err
		}
	}
	{
		file, err := os.Open(path)
		if err != nil {
			log.Print(err)
			return nil, err
		}
		defer file.Close()
		config, _, err := image.DecodeConfig(file)
		if err != nil {
			log.Print(err)
			return nil, err
		}
		i.width = config.Width
		i.height = config.Height
	}

	return &i, nil
} // }}}
